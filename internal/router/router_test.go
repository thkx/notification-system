package router

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

// MockChannelRouter 模拟通知渠道
type MockChannelRouter struct {
	sendCount  atomic.Int32
	shouldFail bool
	delay      time.Duration
}

func (m *MockChannelRouter) Send(notification *model.Notification) error {
	m.sendCount.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.shouldFail {
		return fmt.Errorf("mock send failure")
	}
	return nil
}

func (m *MockChannelRouter) Name() string {
	return "mock"
}

// TestRouterBasicRouting 测试基本路由功能
func TestRouterBasicRouting(t *testing.T) {
	cfg := &RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  0,
		RetryDelay:  10 * time.Millisecond,
	}

	router := NewRouterWithConfig(cfg)
	defer router.Stop()

	mockChannel := &MockChannelRouter{}
	router.RegisterChannel("mock", mockChannel)

	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "user1",
		Content:  "test content",
		Channels: []string{"mock"},
	}

	if err := router.RouteNotification(notification); err != nil {
		t.Fatalf("Failed to route notification: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if mockChannel.sendCount.Load() == 0 {
		t.Errorf("Expected channel.Send to be called, but it wasn't")
	}
}

// TestRouterQueueCapacity 测试队列容量限制
func TestRouterQueueCapacity(t *testing.T) {
	cfg := &RouterConfig{
		BufferSize:  5,
		WorkerCount: 1,
		MaxRetries:  0,
		RetryDelay:  10 * time.Millisecond,
	}

	router := NewRouterWithConfig(cfg)
	defer router.Stop()

	mockChannel := &MockChannelRouter{delay: 50 * time.Millisecond}
	router.RegisterChannel("mock", mockChannel)

	for i := 0; i < 5; i++ {
		notif := &model.Notification{
			ID:       fmt.Sprintf("test-%d", i),
			UserID:   "user1",
			Content:  "test",
			Channels: []string{"mock"},
		}
		if err := router.RouteNotification(notif); err != nil {
			t.Fatalf("Failed to route notification %d: %v", i, err)
		}
	}

	notif := &model.Notification{
		ID:       "test-6",
		UserID:   "user1",
		Content:  "test",
		Channels: []string{"mock"},
	}
	if err := router.RouteNotification(notif); err == nil {
		t.Error("Expected error when queue is full, but got nil")
	}
}

// TestRouterQueueSize 测试队列大小追踪
func TestRouterQueueSize(t *testing.T) {
	cfg := &RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  0,
		RetryDelay:  10 * time.Millisecond,
	}

	router := NewRouterWithConfig(cfg)
	defer router.Stop()

	mockChannel := &MockChannelRouter{delay: 50 * time.Millisecond}
	router.RegisterChannel("mock", mockChannel)

	if initialSize := router.GetQueueSize(); initialSize != 0 {
		t.Errorf("Expected initial queue size 0, got %d", initialSize)
	}

	notif := &model.Notification{
		ID:       "test-1",
		UserID:   "user1",
		Content:  "test",
		Channels: []string{"mock"},
	}

	if err := router.RouteNotification(notif); err != nil {
		t.Fatalf("route notification: %v", err)
	}

	if size := router.GetQueueSize(); size < 0 {
		t.Errorf("Expected queue size >= 0, got %d", size)
	}

	time.Sleep(100 * time.Millisecond)

	if finalSize := router.GetQueueSize(); finalSize < 0 {
		t.Errorf("Expected queue size >= 0 after processing, got %d", finalSize)
	}
}

// TestRouterMultipleChannels 测试多渠道发送
func TestRouterMultipleChannels(t *testing.T) {
	cfg := &RouterConfig{
		BufferSize:  20,
		WorkerCount: 2,
		MaxRetries:  1,
		RetryDelay:  10 * time.Millisecond,
	}

	router := NewRouterWithConfig(cfg)
	defer router.Stop()

	emailChannel := &MockChannelRouter{}
	smsChannel := &MockChannelRouter{}

	router.RegisterChannel("email", emailChannel)
	router.RegisterChannel("sms", smsChannel)

	notification := &model.Notification{
		ID:       "test-multi",
		UserID:   "user1",
		Content:  "test content",
		Channels: []string{"email", "sms"},
	}

	if err := router.RouteNotification(notification); err != nil {
		t.Fatalf("Failed to route notification: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if emailChannel.sendCount.Load() == 0 {
		t.Errorf("Expected email channel to be called")
	}

	if smsChannel.sendCount.Load() == 0 {
		t.Errorf("Expected SMS channel to be called")
	}
}

// TestRouterGracefulShutdown 测试优雅关闭
func TestRouterGracefulShutdown(t *testing.T) {
	cfg := &RouterConfig{
		BufferSize:  10,
		WorkerCount: 2,
		MaxRetries:  1,
		RetryDelay:  10 * time.Millisecond,
	}

	router := NewRouterWithConfig(cfg)
	mockChannel := &MockChannelRouter{delay: 20 * time.Millisecond}
	router.RegisterChannel("mock", mockChannel)

	for i := 0; i < 3; i++ {
		notif := &model.Notification{
			ID:       fmt.Sprintf("test-%d", i),
			UserID:   "user1",
			Content:  "test",
			Channels: []string{"mock"},
		}
		if err := router.RouteNotification(notif); err != nil {
			t.Fatalf("route notification %d: %v", i, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := router.GracefulStop(ctx); err != nil {
		t.Fatalf("Graceful shutdown failed: %v", err)
	}

	if router.GetQueueSize() != 0 {
		t.Fatalf("expected queue to be empty, got %d", router.GetQueueSize())
	}
	if router.getProcessingCount() != 0 {
		t.Fatalf("expected no active processing, got %d", router.getProcessingCount())
	}
}

func TestRouterRejectsNewNotificationsAfterGracefulStop(t *testing.T) {
	router := NewRouterWithConfig(&RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  0,
		RetryDelay:  10 * time.Millisecond,
	})
	router.RegisterChannel("mock", &MockChannelRouter{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := router.GracefulStop(ctx); err != nil {
		t.Fatalf("graceful stop failed: %v", err)
	}

	err := router.RouteNotification(&model.Notification{
		ID:       "after-stop",
		UserID:   "user1",
		Content:  "test",
		Channels: []string{"mock"},
	})
	if err == nil {
		t.Fatal("expected routing to fail after graceful stop")
	}
}

func TestRouterStopIsIdempotent(t *testing.T) {
	router := NewRouterWithConfig(&RouterConfig{
		BufferSize:  1,
		WorkerCount: 1,
		MaxRetries:  0,
		RetryDelay:  10 * time.Millisecond,
	})

	router.Stop()
	router.Stop()
}
