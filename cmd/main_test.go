package main

import (
	"testing"
	"time"

	"github.com/thkx/notification-system/config"
)

func TestBuildApplication(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           8080,
			ReadTimeoutMs:  5000,
			WriteTimeoutMs: 10000,
			IdleTimeoutMs:  60000,
			MaxBodyBytes:   1 << 20,
		},
		Router: config.RouterConfig{
			BufferSize:   10,
			WorkerCount:  1,
			MaxRetries:   1,
			RetryDelayMs: 10,
		},
		Channels: config.ChannelsConfig{
			Email:       config.ChannelConfig{Enabled: true},
			SMS:         config.ChannelConfig{Enabled: true},
			InApp:       config.ChannelConfig{Enabled: true},
			SocialMedia: config.ChannelConfig{Enabled: true},
		},
		Metrics: config.MetricsConfig{
			MaxFailureRate:      0.2,
			MaxQueueUtilization: 0.8,
			MaxProcessingTime:   5000,
		},
		Distribution: config.DistributionConfig{DeduplicationTTL: 60},
		Store:        config.StoreConfig{Type: "memory"},
		Environment:  "test",
	}

	app, err := buildApplication(cfg)
	if err != nil {
		t.Fatalf("build application: %v", err)
	}
	defer app.router.Stop()

	if app.gateway == nil || app.httpServer == nil {
		t.Fatal("expected application services to be initialized")
	}

	if len(app.registeredChannels) != 4 {
		t.Fatalf("expected 4 registered channels, got %d", len(app.registeredChannels))
	}

	if err := app.orderService.ProcessOrder("test-123", "test-user"); err != nil {
		t.Fatalf("order service failed: %v", err)
	}

	if err := app.paymentService.ProcessPayment("test-456", "test-user"); err != nil {
		t.Fatalf("payment service failed: %v", err)
	}

	result, err := app.gateway.SendBatchNotifications(defaultDemoNotifications())
	if err != nil {
		t.Fatalf("batch notifications failed: %v", err)
	}
	if result == nil || result.Total != 2 {
		t.Fatalf("expected batch result total 2, got %#v", result)
	}

	time.Sleep(100 * time.Millisecond)
	if queueSize := app.router.GetQueueSize(); queueSize < 0 {
		t.Fatalf("expected queue size >= 0, got %d", queueSize)
	}
}

func TestShouldRunStartupDemo(t *testing.T) {
	t.Setenv("NOTIFICATION_RUN_DEMO", "true")
	if !shouldRunStartupDemo() {
		t.Fatal("expected startup demo to be enabled")
	}

	t.Setenv("NOTIFICATION_RUN_DEMO", "false")
	if shouldRunStartupDemo() {
		t.Fatal("expected startup demo to be disabled")
	}
}
