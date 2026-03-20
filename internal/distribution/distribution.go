package distribution

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thkx/notification-system/internal/router"
	"github.com/thkx/notification-system/pkg/errors"
	"github.com/thkx/notification-system/pkg/logger"
	"github.com/thkx/notification-system/pkg/model"
	"github.com/thkx/notification-system/pkg/security"
)

// ProcessedNotificationCache 已处理通知缓存，用于防止重复处理
type ProcessedNotificationCache struct {
	processed map[string]time.Time // 通知ID -> 处理时间
	ttl       time.Duration
	mutex     sync.RWMutex
}

// NewProcessedNotificationCache 创建新的缓存实例
func NewProcessedNotificationCache(ttl time.Duration) *ProcessedNotificationCache {
	cache := &ProcessedNotificationCache{
		processed: make(map[string]time.Time),
		ttl:       ttl,
	}

	// 启动后台清理协程，每分钟清理过期项
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			cache.cleanup()
		}
	}()

	return cache
}

// IsProcessed 检查通知是否已处理
func (c *ProcessedNotificationCache) IsProcessed(id string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	processedAt, exists := c.processed[id]
	if !exists {
		return false
	}

	// 检查是否过期
	return time.Since(processedAt) < c.ttl
}

// Mark 标记通知已处理
func (c *ProcessedNotificationCache) Mark(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.processed[id] = time.Now()
}

// cleanup 清理过期的缓存项
func (c *ProcessedNotificationCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for id, processedAt := range c.processed {
		if now.Sub(processedAt) > c.ttl {
			delete(c.processed, id)
		}
	}

	logger.Debug("Cache cleanup completed, remaining items: %d", len(c.processed))
}

// Distribution 通知分发组件，负责通知的验证、规范化、优先级设置、模板应用和调度
type Distribution struct {
	router *router.Router              // 路由器，用于将处理后的通知路由到相应的渠道
	cache  *ProcessedNotificationCache // 已处理通知缓存，用于防止重复
}

// NewDistribution 创建一个新的Distribution实例
// @param router 通知路由器实例
// @return 新创建的Distribution实例
func NewDistribution(router *router.Router) *Distribution {
	return &Distribution{
		router: router,
		cache:  NewProcessedNotificationCache(1 * time.Minute), // TTL=1分钟
	}
}

// NewDistributionWithTTL 创建一个带有自定义去重缓存TTL的Distribution实例
// @param router 通知路由器实例
// @param deduplicationTTL 去重缓存TTL(秒)
// @return 新创建的Distribution实例
func NewDistributionWithTTL(router *router.Router, deduplicationTTL int) *Distribution {
	return &Distribution{
		router: router,
		cache:  NewProcessedNotificationCache(time.Duration(deduplicationTTL) * time.Second),
	}
}

// ProcessNotification 处理通知的主流程
// @param notification 待处理的通知
// @return 处理过程中的错误
func (d *Distribution) ProcessNotification(notification *model.Notification) error {
	logger.Info("Processing notification: ID=%s, UserID=%s, Type=%s",
		notification.ID, notification.UserID, notification.Type)

	// 0. 检查重复（去重与幂等性）
	if d.cache.IsProcessed(notification.ID) {
		logger.Warn("Duplicate notification detected: ID=%s, skipping", notification.ID)
		return errors.DistributionError("Duplicate notification", "Notification already processed", nil)
	}

	// 1. 验证通知的合法性
	if err := d.validateNotification(notification); err != nil {
		logger.Error("Notification validation failed: ID=%s, Error=%v", notification.ID, err)
		return err
	}

	// 2. 安全检查和验证
	validationResult := security.ValidateNotification(notification.Content)
	if !validationResult.Valid {
		err := errors.ValidationError("Notification content validation failed", validationResult.Message, nil)
		logger.Error("Notification content validation failed: ID=%s, Error=%v", notification.ID, err)
		return err
	}

	// 3. 清理通知内容
	notification.Content = security.SanitizeContent(notification.Content)
	logger.Info("Notification content sanitized: ID=%s", notification.ID)

	// 4. 规范化通知数据
	normalizedNotification := d.normalizeNotification(notification)
	logger.Info("Notification normalized: ID=%s", notification.ID)

	// 5. 设置通知优先级
	prioritizedNotification := d.setPriority(normalizedNotification)
	logger.Info("Notification priority set: ID=%s, Priority=%d", notification.ID, prioritizedNotification.Priority)

	// 6. 应用通知模板
	templatedNotification := d.applyTemplate(prioritizedNotification)
	logger.Info("Notification template applied: ID=%s", notification.ID)

	// 7. 调度通知发送时间
	scheduledNotification := d.scheduleNotification(templatedNotification)
	logger.Info("Notification scheduled: ID=%s", notification.ID)

	// 8. 将处理后的通知路由到相应的渠道
	if err := d.router.RouteNotification(scheduledNotification); err != nil {
		logger.Error("Failed to route notification: ID=%s, Error=%v", notification.ID, err)
		return err
	}

	// 9. 标记通知已处理（防止重复）
	d.cache.Mark(notification.ID)

	logger.Info("Notification processed successfully: ID=%s", notification.ID)
	return nil
}

// validateNotification 验证通知的合法性
// @param notification 待验证的通知
// @return 验证错误，如果验证通过则返回nil
func (d *Distribution) validateNotification(notification *model.Notification) error {
	// 这里可以添加更复杂的验证逻辑
	// 例如：验证用户ID是否存在，通知内容是否合法等
	if notification.UserID == "" {
		// 示例：用户ID为空时的处理
		logger.Warn("Notification has empty UserID: ID=%s", notification.ID)
	}

	if len(notification.Channels) == 0 {
		logger.Warn("Notification has no channels: ID=%s", notification.ID)
	}

	return nil
}

// normalizeNotification 规范化通知数据
// @param notification 待规范化的通知
// @return 规范化后的通知
func (d *Distribution) normalizeNotification(notification *model.Notification) *model.Notification {
	// 规范化渠道名称，确保小写
	for i, channel := range notification.Channels {
		notification.Channels[i] = strings.ToLower(channel)
	}

	// 确保通知ID不为空
	if notification.ID == "" {
		notification.ID = fmt.Sprintf("notif-%d", time.Now().UnixNano())
	}

	// 确保优先级在合理范围内
	if notification.Priority < 0 {
		notification.Priority = 0
	} else if notification.Priority > 5 {
		notification.Priority = 5
	}

	return notification
}

// setPriority 设置通知的优先级
// @param notification 待设置优先级的通知
// @return 设置优先级后的通知
func (d *Distribution) setPriority(notification *model.Notification) *model.Notification {
	// 如果优先级未设置，设置默认优先级为1
	if notification.Priority == 0 {
		notification.Priority = 1
	}

	// 根据通知类型设置默认优先级
	switch notification.Type {
	case "urgent":
		if notification.Priority < 4 {
			notification.Priority = 4
		}
	case "important":
		if notification.Priority < 3 {
			notification.Priority = 3
		}
	case "promotion":
		if notification.Priority > 2 {
			notification.Priority = 2
		}
	case "newsletter":
		if notification.Priority > 1 {
			notification.Priority = 1
		}
	}

	return notification
}

// applyTemplate 应用通知模板
// @param notification 待应用模板的通知
// @return 应用模板后的通知
func (d *Distribution) applyTemplate(notification *model.Notification) *model.Notification {
	// 根据通知类型应用不同的模板
	switch notification.Type {
	case "order":
		notification.Content = fmt.Sprintf("您的订单 %s 已处理完成，感谢您的购买！", notification.ID)
	case "payment":
		notification.Content = fmt.Sprintf("您的支付 %s 已成功完成，交易已确认。", notification.ID)
	case "promotion":
		notification.Content = fmt.Sprintf("【限时优惠】%s", notification.Content)
	case "newsletter":
		notification.Content = fmt.Sprintf("【每周通讯】%s", notification.Content)
	case "urgent":
		notification.Content = fmt.Sprintf("【紧急通知】%s", notification.Content)
	}

	return notification
}

// scheduleNotification 调度通知的发送时间
// @param notification 待调度的通知
// @return 调度后的通知
func (d *Distribution) scheduleNotification(notification *model.Notification) *model.Notification {
	// 如果未设置调度时间，使用当前时间
	if notification.Scheduled == "" {
		notification.Scheduled = time.Now().Format(time.RFC3339)
	}

	// 这里可以添加更复杂的调度逻辑
	// 例如：根据用户设置的接收时间，延迟发送通知

	return notification
}
