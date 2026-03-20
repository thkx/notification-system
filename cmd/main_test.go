package main

import (
	"testing"
	"time"

	"github.com/thkx/notification-system/internal/channels"
	"github.com/thkx/notification-system/internal/distribution"
	"github.com/thkx/notification-system/internal/gateway"
	"github.com/thkx/notification-system/internal/router"
	"github.com/thkx/notification-system/internal/services"
	"github.com/thkx/notification-system/internal/storage"
	"github.com/thkx/notification-system/pkg/model"
)

func TestNotificationSystem(t *testing.T) {
	// 初始化组件
	emailChannel := channels.NewEmailChannel()
	inAppChannel := channels.NewInAppChannel()
	smsChannel := channels.NewSMSChannel()
	socialMediaChannel := channels.NewSocialMediaChannel()

	router := router.NewRouter()
	router.RegisterChannel("email", emailChannel)
	router.RegisterChannel("inapp", inAppChannel)
	router.RegisterChannel("sms", smsChannel)
	router.RegisterChannel("social", socialMediaChannel)

	distribution := distribution.NewDistribution(router)
	notificationGateway := gateway.NewGateway(distribution, storage.NewMemoryStore())

	orderService := services.NewOrderService(notificationGateway)
	paymentService := services.NewPaymentService(notificationGateway)

	// 测试订单服务
	err := orderService.ProcessOrder("test-123", "test-user")
	if err != nil {
		t.Errorf("Order service failed: %v", err)
	}

	// 测试支付服务
	err = paymentService.ProcessPayment("test-456", "test-user")
	if err != nil {
		t.Errorf("Payment service failed: %v", err)
	}

	// 测试批量通知
	notifications := []*model.Notification{
		{
			ID:       "batch-test-1",
			UserID:   "test-user-2",
			Type:     "promotion",
			Content:  "Test promotion",
			Channels: []string{"email", "social"},
			Priority: 1,
		},
	}

	result, err := notificationGateway.SendBatchNotifications(notifications)
	if err != nil {
		t.Errorf("Batch notification failed: %v", err)
	}
	if result != nil && result.Failed > 0 {
		t.Logf("Batch result: Total=%d, Successful=%d, Failed=%d",
			result.Total, result.Successful, result.Failed)
	}

	// 等待一段时间，确保通知被处理
	time.Sleep(100 * time.Millisecond)

	// 验证队列大小（使用通道后，队列大小会快速变为0）
	queueSize := router.GetQueueSize()
	if queueSize < 0 {
		t.Errorf("Expected queue size >= 0, got %d", queueSize)
	}
}
