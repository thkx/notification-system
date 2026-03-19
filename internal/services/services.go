package services

import (
	"github.com/thkx/notification-system/pkg/model"

	"github.com/thkx/notification-system/internal/gateway"
)

// OrderService 订单服务，处理订单相关的业务逻辑
type OrderService struct {
	gateway *gateway.Gateway // 通知网关，用于发送订单相关通知
}

// NewOrderService 创建一个新的OrderService实例
// @param gateway 通知网关实例
// @return 新创建的OrderService实例
func NewOrderService(gateway *gateway.Gateway) *OrderService {
	return &OrderService{
		gateway: gateway,
	}
}

// ProcessOrder 处理订单
// @param orderID 订单ID
// @param userID 用户ID
// @return 处理过程中的错误
func (s *OrderService) ProcessOrder(orderID string, userID string) error {
	// 处理订单逻辑
	// 这里可以添加实际的订单处理逻辑，如更新订单状态、库存等

	// 创建订单通知
	notification := &model.Notification{
		ID:       "order-" + orderID,
		UserID:   userID,
		Type:     "order",
		Content:  "Your order " + orderID + " has been processed successfully",
		Channels: []string{"email", "inapp"},
		Priority: 2,
	}

	// 发送通知
	return s.gateway.SendNotification(notification)
}

// PaymentService 支付服务，处理支付相关的业务逻辑
type PaymentService struct {
	gateway *gateway.Gateway // 通知网关，用于发送支付相关通知
}

// NewPaymentService 创建一个新的PaymentService实例
// @param gateway 通知网关实例
// @return 新创建的PaymentService实例
func NewPaymentService(gateway *gateway.Gateway) *PaymentService {
	return &PaymentService{
		gateway: gateway,
	}
}

// ProcessPayment 处理支付
// @param paymentID 支付ID
// @param userID 用户ID
// @return 处理过程中的错误
func (s *PaymentService) ProcessPayment(paymentID string, userID string) error {
	// 处理支付逻辑
	// 这里可以添加实际的支付处理逻辑，如验证支付信息、更新支付状态等

	// 创建支付通知
	notification := &model.Notification{
		ID:       "payment-" + paymentID,
		UserID:   userID,
		Type:     "payment",
		Content:  "Your payment " + paymentID + " has been processed successfully",
		Channels: []string{"sms", "email"},
		Priority: 3,
	}

	// 发送通知
	return s.gateway.SendNotification(notification)
}
