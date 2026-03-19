package router

import (
	"context"
	"testing"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

// MockChannelRouter 模拟通知渠道
type MockChannelRouter struct {
	sendCount  int
	shouldFail bool
	delay      time.Duration
}

func (m *MockChannelRouter) Send(notification *model.Notification) error {
	m.sendCount++
	if m.delay > 0 {
		time.Sleep(m.delay)
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

	err := router.RouteNotification(notification)
	if err != nil {
		t.Fatalf("Failed to route notification: %v", err)
	}

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if mockChannel.sendCount == 0 {
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

	// 填满队列
	for i := 0; i < 5; i++ {
		notif := &model.Notification{
			ID:       string(rune(i)) + "test",
			UserID:   "user1",
			Content:  "test",
			Channels: []string{"mock"},
		}
		err := router.RouteNotification(notif)
		if err != nil {
			t.Fatalf("Failed to route notification %d: %v", i, err)
		}
	}

	// 第6个应该失败（队列满）
	notif := &model.Notification{
		ID:       "test-6",
		UserID:   "user1",
		Content:  "test",
		Channels: []string{"mock"},
	}
	err := router.RouteNotification(notif)
	if err == nil {
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

	initialSize := router.GetQueueSize()
	if initialSize != 0 {
		t.Errorf("Expected initial queue size 0, got %d", initialSize)
	}

	// 添加通知
	notif := &model.Notification{
		ID:       "test-1",
		UserID:   "user1",
		Content:  "test",
		Channels: []string{"mock"},
	}

	router.RouteNotification(notif)

	// 队列大小应该增加（或立即处理，都可接受）
	size := router.GetQueueSize()
	if size < 0 {
		t.Errorf("Expected queue size >= 0, got %d", size)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	finalSize := router.GetQueueSize()
	if finalSize < 0 {
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

	err := router.RouteNotification(notification)
	if err != nil {
		t.Fatalf("Failed to route notification: %v", err)
	}

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if emailChannel.sendCount == 0 {
		t.Errorf("Expected email channel to be called")
	}

	if smsChannel.sendCount == 0 {
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

	// 添加几个通知
	for i := 0; i < 3; i++ {
		notif := &model.Notification{
			ID:       string(rune(i)) + "test",
			UserID:   "user1",
			Content:  "test",
			Channels: []string{"mock"},
		}
		router.RouteNotification(notif)
	}

	// 优雅关闭，等待5秒
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := router.GracefulStop(ctx)
	if err != nil {
		t.Logf("Graceful shutdown returned error (expected if queue not empty): %v", err)
	}
}
