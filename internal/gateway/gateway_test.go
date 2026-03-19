package gateway

import (
	"testing"

	"github.com/thkx/notification-system/internal/distribution"
	"github.com/thkx/notification-system/internal/router"
	"github.com/thkx/notification-system/pkg/model"
)

// MockChannelGateway 模拟通知渠道
type MockChannelGateway struct {
	sendCount int
}

func (m *MockChannelGateway) Send(notification *model.Notification) error {
	m.sendCount++
	return nil
}

func (m *MockChannelGateway) Name() string {
	return "mock"
}

// TestGatewaySingleNotification 测试单个通知发送
func TestGatewaySingleNotification(t *testing.T) {
	// 初始化依赖
	mockChannel := &MockChannelGateway{}
	notificationRouter := router.NewRouter()
	notificationRouter.RegisterChannel("mock", mockChannel)

	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "user1",
		Type:     "test",
		Content:  "test content",
		Channels: []string{"mock"},
		Priority: 1,
	}

	err := gateway.SendNotification(notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}
}

// TestGatewayNilNotification 测试nil通知
func TestGatewayNilNotification(t *testing.T) {
	notificationRouter := router.NewRouter()
	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	err := gateway.SendNotification(nil)
	if err == nil {
		t.Error("Expected error for nil notification")
	}
}

// TestGatewayEmptyUserID 测试空UserID
func TestGatewayEmptyUserID(t *testing.T) {
	notificationRouter := router.NewRouter()
	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "", // 空
		Type:     "test",
		Content:  "test content",
		Channels: []string{"mock"},
	}

	err := gateway.SendNotification(notification)
	if err == nil {
		t.Error("Expected error for empty UserID")
	}
}

// TestGatewayNoChannels 测试没有渠道
func TestGatewayNoChannels(t *testing.T) {
	notificationRouter := router.NewRouter()
	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "user1",
		Type:     "test",
		Content:  "test content",
		Channels: []string{}, // 空
	}

	err := gateway.SendNotification(notification)
	if err == nil {
		t.Error("Expected error for no channels")
	}
}

// TestGatewayBatchNotifications 测试批量发送
func TestGatewayBatchNotifications(t *testing.T) {
	mockChannel := &MockChannelGateway{}
	notificationRouter := router.NewRouter()
	notificationRouter.RegisterChannel("mock", mockChannel)

	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	notifications := []*model.Notification{
		{
			ID:       "batch-1",
			UserID:   "user1",
			Type:     "test",
			Content:  "test 1",
			Channels: []string{"mock"},
		},
		{
			ID:       "batch-2",
			UserID:   "user2",
			Type:     "test",
			Content:  "test 2",
			Channels: []string{"mock"},
		},
		{
			ID:       "batch-3",
			UserID:   "user3",
			Type:     "test",
			Content:  "test 3",
			Channels: []string{"mock"},
		},
	}

	result, err := gateway.SendBatchNotifications(notifications)
	if err != nil {
		t.Fatalf("Failed to send batch notifications: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Total != 3 {
		t.Errorf("Expected total=3, got %d", result.Total)
	}

	if result.Successful != 3 {
		t.Errorf("Expected successful=3, got %d", result.Successful)
	}

	if result.Failed != 0 {
		t.Errorf("Expected failed=0, got %d", result.Failed)
	}
}

// TestGatewayBatchEmptyList 测试空批列表
func TestGatewayBatchEmptyList(t *testing.T) {
	notificationRouter := router.NewRouter()
	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	result, err := gateway.SendBatchNotifications([]*model.Notification{})
	if err != nil {
		t.Fatalf("Failed on empty list: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("Expected total=0, got %d", result.Total)
	}
}

// TestGatewayBatchPartialFailure 测试批量部分失败
func TestGatewayBatchPartialFailure(t *testing.T) {
	mockChannel := &MockChannelGateway{}
	notificationRouter := router.NewRouter()
	notificationRouter.RegisterChannel("mock", mockChannel)

	dist := distribution.NewDistribution(notificationRouter)
	gateway := NewGateway(dist)

	notifications := []*model.Notification{
		{
			ID:       "batch-1",
			UserID:   "user1",
			Type:     "test",
			Content:  "test 1",
			Channels: []string{"mock"},
		},
		{
			ID:       "batch-2",
			UserID:   "", // 无效
			Type:     "test",
			Content:  "test 2",
			Channels: []string{"mock"},
		},
		{
			ID:       "batch-3",
			UserID:   "user3",
			Type:     "test",
			Content:  "test 3",
			Channels: []string{"mock"},
		},
	}

	result, _ := gateway.SendBatchNotifications(notifications)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Total != 3 {
		t.Errorf("Expected total=3, got %d", result.Total)
	}

	if result.Failed != 1 {
		t.Errorf("Expected failed=1, got %d", result.Failed)
	}

	if len(result.FailedIDs) != 1 || result.FailedIDs[0] != "batch-2" {
		t.Errorf("Expected failed IDs to contain batch-2, got %v", result.FailedIDs)
	}
}
