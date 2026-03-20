package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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

func main() {
	// 捕获异常，确保系统优雅退出
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("System recovered from panic:", r)
		}
	}()

	// 加载配置
	cfg := config.LoadConfig()

	// 初始化metrics配置
	metrics.InitMetricsWithConfig(
		cfg.Metrics.MaxFailureRate,
		cfg.Metrics.MaxQueueUtilization,
		cfg.Metrics.MaxProcessingTime,
	)

	// 初始化依赖注入容器
	container := di.NewContainer()

	// 注入配置
	container.Register("config", cfg)

	// 初始化组件
	analyticsService := analytics.NewAnalytics()
	container.Register("analytics", analyticsService)

	// 初始化渠道
	emailChannel := channels.NewEmailChannel()
	inAppChannel := channels.NewInAppChannel()
	smsChannel := channels.NewSMSChannel()
	socialMediaChannel := channels.NewSocialMediaChannel()

	// 注入渠道
	container.Register("emailChannel", emailChannel)
	container.Register("inAppChannel", inAppChannel)
	container.Register("smsChannel", smsChannel)
	container.Register("socialMediaChannel", socialMediaChannel)

	// 初始化路由器，使用配置中的参数
	routerCfg := &router.RouterConfig{
		BufferSize:  cfg.Router.BufferSize,
		WorkerCount: cfg.Router.WorkerCount,
		MaxRetries:  cfg.Router.MaxRetries,
		RetryDelay:  time.Duration(cfg.Router.RetryDelayMs) * time.Millisecond,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	if cfg.Channels.Email.Enabled {
		notificationRouter.RegisterChannel("email", emailChannel)
	}
	if cfg.Channels.InApp.Enabled {
		notificationRouter.RegisterChannel("inapp", inAppChannel)
	}
	if cfg.Channels.SMS.Enabled {
		notificationRouter.RegisterChannel("sms", smsChannel)
	}
	if cfg.Channels.SocialMedia.Enabled {
		notificationRouter.RegisterChannel("social", socialMediaChannel)
	}
	container.Register("router", notificationRouter)

	// 初始化持久化存储（可选择 memory/postgres）
	var store storage.NotificationStore
	if cfg.Store.Type == "postgres" {
		pgStore, err := storage.NewPostgresStore(cfg.Store.DSN)
		if err != nil {
			fmt.Printf("Failed to initialize PostgreSQL store: %v, falling back to memory store\n", err)
			store = storage.NewMemoryStore()
		} else {
			store = pgStore
		}
	} else {
		store = storage.NewMemoryStore()
	}
	container.Register("store", store)

	// 初始化分发器
	distribution := distribution.NewDistributionWithTTL(notificationRouter, cfg.Distribution.DeduplicationTTL)
	container.Register("distribution", distribution)

	// 初始化网关
	notificationGateway := gateway.NewGateway(distribution, store)
	container.Register("gateway", notificationGateway)

	// 初始化HTTP服务器
	httpServer := api.NewServer(notificationGateway, cfg.Server.Port)
	container.Register("httpServer", httpServer)

	// 启动HTTP服务器
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatalf("HTTP server failed to start: %v", err)
		}
	}()

	// 初始化业务服务
	orderService := services.NewOrderService(notificationGateway)
	paymentService := services.NewPaymentService(notificationGateway)
	container.Register("orderService", orderService)
	container.Register("paymentService", paymentService)

	// 测试通知发送
	fmt.Println("Testing notification system...")

	// 测试订单服务
	err := orderService.ProcessOrder("123", "user123")
	if err != nil {
		fmt.Println("Error sending order notification:", err)
	}

	// 测试支付服务
	err = paymentService.ProcessPayment("456", "user123")
	if err != nil {
		fmt.Println("Error sending payment notification:", err)
	}

	// 测试批量通知
	notifications := []*model.Notification{
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

	result, err := notificationGateway.SendBatchNotifications(notifications)
	if err != nil {
		fmt.Printf("Error sending batch notifications: %v\n", err)
	}
	if result != nil && result.Failed > 0 {
		fmt.Printf("Batch result: Total=%d, Successful=%d, Failed=%d\n",
			result.Total, result.Successful, result.Failed)
	}

	fmt.Println("Notification system test completed!")
	fmt.Println("Queue size:", notificationRouter.GetQueueSize())
	fmt.Println("Event count:", analyticsService.GetEventCount())

	// 输出监控指标
	metrics := metrics.GetMetrics()
	fmt.Println("\nMetrics:")
	fmt.Println("Total notifications:", metrics.GetTotal())
	fmt.Println("Successful notifications:", metrics.GetSuccessful())
	fmt.Println("Failed notifications:", metrics.GetFailed())
	fmt.Println("Channel metrics:")
	for channel, metric := range metrics.GetChannelMetrics() {
		fmt.Printf("  %s: Total=%d, Successful=%d, Failed=%d\n",
			channel, metric.Total, metric.Successful, metric.Failed)
	}

	// 等待系统运行，保持HTTP服务器活跃
	fmt.Println("Notification system started successfully!")
	fmt.Printf("HTTP server running on port %d\n", cfg.Server.Port)
	fmt.Println("API endpoints:")
	fmt.Println("  GET  /health - Health check")
	fmt.Println("  POST /api/notifications - Send single notification")
	fmt.Println("  POST /api/notifications/batch - Send batch notifications")
	fmt.Println("  GET  /ws - WebSocket connection for real-time notifications")
	fmt.Println("\nPress Ctrl+C to shutdown gracefully...")

	// 设置信号处理，实现优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待中断信号
	<-sigChan

	fmt.Println("\nShutdown signal received, performing graceful shutdown...")

	// 等待队列处理完成（最多30秒）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := notificationRouter.GracefulStop(ctx); err != nil {
		log.Printf("Router graceful stop error: %v", err)
	}

	fmt.Println("Shutdown complete, exiting.")
}
