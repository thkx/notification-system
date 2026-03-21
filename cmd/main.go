package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/thkx/notification-system/config"
	"github.com/thkx/notification-system/internal/analytics"
	"github.com/thkx/notification-system/internal/api"
	"github.com/thkx/notification-system/internal/channels"
	"github.com/thkx/notification-system/internal/distribution"
	"github.com/thkx/notification-system/internal/gateway"
	"github.com/thkx/notification-system/internal/router"
	"github.com/thkx/notification-system/internal/services"
	"github.com/thkx/notification-system/internal/storage"
	"github.com/thkx/notification-system/pkg/di"
	"github.com/thkx/notification-system/pkg/metrics"
	"github.com/thkx/notification-system/pkg/model"
)

type application struct {
	config             *config.Config
	container          *di.Container
	analytics          *analytics.Analytics
	router             *router.Router
	store              storage.NotificationStore
	distribution       *distribution.Distribution
	gateway            *gateway.Gateway
	httpServer         *api.Server
	orderService       *services.OrderService
	paymentService     *services.PaymentService
	demoNotifications  []*model.Notification
	registeredChannels []string
}

type ioCloser interface {
	Close() error
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("System recovered from panic:", r)
		}
	}()

	if err := run(); err != nil {
		log.Fatalf("notification system exited with error: %v", err)
	}
}

func run() error {
	cfg := config.LoadConfig()
	app, err := buildApplication(cfg)
	if err != nil {
		return err
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := app.httpServer.Start(); err != nil {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	if shouldRunStartupDemo() {
		runStartupDemo(app)
	}

	printStartupSummary(app)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		fmt.Printf("\nShutdown signal received (%s), performing graceful shutdown...\n", sig)
	case err := <-serverErrCh:
		if err != nil {
			return fmt.Errorf("http server failed: %w", err)
		}
		return fmt.Errorf("http server stopped unexpectedly")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server graceful shutdown error: %v", err)
	}
	if err := app.router.GracefulStop(ctx); err != nil {
		log.Printf("Router graceful stop error: %v", err)
	}
	if closer, ok := app.store.(ioCloser); ok {
		if err := closer.Close(); err != nil {
			log.Printf("Store close error: %v", err)
		}
	}

	fmt.Println("Shutdown complete, exiting.")
	return nil
}

func buildApplication(cfg *config.Config) (*application, error) {
	metrics.InitMetricsWithConfig(
		cfg.Metrics.MaxFailureRate,
		cfg.Metrics.MaxQueueUtilization,
		cfg.Metrics.MaxProcessingTime,
	)

	container := di.NewContainer()
	container.Register("config", cfg)

	analyticsService := analytics.NewAnalytics()
	container.Register("analytics", analyticsService)

	emailChannel := channels.NewEmailChannel()
	inAppChannel := channels.NewInAppChannel()
	smsChannel := channels.NewSMSChannel()
	socialMediaChannel := channels.NewSocialMediaChannel()

	container.Register("emailChannel", emailChannel)
	container.Register("inAppChannel", inAppChannel)
	container.Register("smsChannel", smsChannel)
	container.Register("socialMediaChannel", socialMediaChannel)

	routerCfg := &router.RouterConfig{
		BufferSize:  cfg.Router.BufferSize,
		WorkerCount: cfg.Router.WorkerCount,
		MaxRetries:  cfg.Router.MaxRetries,
		RetryDelay:  time.Duration(cfg.Router.RetryDelayMs) * time.Millisecond,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)

	registeredChannels := make([]string, 0, 4)
	if cfg.Channels.Email.Enabled {
		notificationRouter.RegisterChannel("email", emailChannel)
		registeredChannels = append(registeredChannels, "email")
	}
	if cfg.Channels.InApp.Enabled {
		notificationRouter.RegisterChannel("inapp", inAppChannel)
		registeredChannels = append(registeredChannels, "inapp")
	}
	if cfg.Channels.SMS.Enabled {
		notificationRouter.RegisterChannel("sms", smsChannel)
		registeredChannels = append(registeredChannels, "sms")
	}
	if cfg.Channels.SocialMedia.Enabled {
		notificationRouter.RegisterChannel("social", socialMediaChannel)
		registeredChannels = append(registeredChannels, "social")
	}
	container.Register("router", notificationRouter)

	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	container.Register("store", store)

	dist := distribution.NewDistributionWithTTL(notificationRouter, cfg.Distribution.DeduplicationTTL)
	container.Register("distribution", dist)

	notificationGateway := gateway.NewGateway(dist, store)
	container.Register("gateway", notificationGateway)

	httpServer := api.NewServer(notificationGateway, cfg.Server.Port)
	container.Register("httpServer", httpServer)

	orderService := services.NewOrderService(notificationGateway)
	paymentService := services.NewPaymentService(notificationGateway)
	container.Register("orderService", orderService)
	container.Register("paymentService", paymentService)

	return &application{
		config:             cfg,
		container:          container,
		analytics:          analyticsService,
		router:             notificationRouter,
		store:              store,
		distribution:       dist,
		gateway:            notificationGateway,
		httpServer:         httpServer,
		orderService:       orderService,
		paymentService:     paymentService,
		demoNotifications:  defaultDemoNotifications(),
		registeredChannels: registeredChannels,
	}, nil
}

func newStore(cfg *config.Config) (storage.NotificationStore, error) {
	if cfg.Store.Type != "postgres" {
		return storage.NewMemoryStore(), nil
	}

	pgStore, err := storage.NewPostgresStore(cfg.Store.DSN)
	if err != nil {
		fmt.Printf("Failed to initialize PostgreSQL store: %v, falling back to memory store\n", err)
		return storage.NewMemoryStore(), nil
	}

	return pgStore, nil
}

func shouldRunStartupDemo() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("NOTIFICATION_RUN_DEMO")))
	return raw == "1" || raw == "true" || raw == "yes"
}

func runStartupDemo(app *application) {
	fmt.Println("Running startup demo...")

	if err := app.orderService.ProcessOrder("123", "user123"); err != nil {
		fmt.Println("Error sending order notification:", err)
	}

	if err := app.paymentService.ProcessPayment("456", "user123"); err != nil {
		fmt.Println("Error sending payment notification:", err)
	}

	result, err := app.gateway.SendBatchNotifications(app.demoNotifications)
	if err != nil {
		fmt.Printf("Error sending batch notifications: %v\n", err)
	}
	if result != nil && result.Failed > 0 {
		fmt.Printf("Batch result: Total=%d, Successful=%d, Failed=%d\n",
			result.Total, result.Successful, result.Failed)
	}

	fmt.Println("Startup demo completed!")
}

func printStartupSummary(app *application) {
	fmt.Println("Notification system started successfully!")
	fmt.Printf("Environment: %s\n", app.config.Environment)
	fmt.Printf("HTTP server running on port %d\n", app.config.Server.Port)
	fmt.Printf("Registered channels: %s\n", strings.Join(app.registeredChannels, ", "))
	fmt.Printf("Storage backend: %s\n", app.config.Store.Type)
	fmt.Println("API endpoints:")
	fmt.Println("  GET  /health - Health check")
	fmt.Println("  POST /api/notifications - Send single notification")
	fmt.Println("  GET  /api/notifications - List notifications")
	fmt.Println("  GET  /api/notifications/{id} - Query notification by ID")
	fmt.Println("  POST /api/notifications/batch - Send batch notifications")
	fmt.Println("  GET  /ws - WebSocket connection for real-time notifications")
	fmt.Println("\nSet NOTIFICATION_RUN_DEMO=true to run startup demo notifications.")
	fmt.Println("Press Ctrl+C to shutdown gracefully...")
}

func defaultDemoNotifications() []*model.Notification {
	return []*model.Notification{
		{
			ID:       "batch-1",
			UserID:   "user456",
			Type:     "promotion",
			Content:  "Special promotion for you!",
			Channels: []string{"email", "social"},
			Priority: 1,
		},
		{
			ID:       "batch-2",
			UserID:   "user789",
			Type:     "newsletter",
			Content:  "Weekly newsletter",
			Channels: []string{"email"},
			Priority: 1,
		},
	}
}
