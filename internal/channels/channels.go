package channels

import (
	"fmt"
	"sync"

	"github.com/thkx/notification-system/pkg/errors"
	"github.com/thkx/notification-system/pkg/logger"
	"github.com/thkx/notification-system/pkg/metrics"
	"github.com/thkx/notification-system/pkg/model"
	"github.com/thkx/notification-system/pkg/retry"
)

// Channel 通知渠道接口，定义了发送通知的方法
type Channel interface {
	// Send 发送通知
	// @param notification 待发送的通知
	// @return 发送过程中的错误
	Send(notification *model.Notification) error

	// Name 获取渠道名称
	// @return 渠道名称
	Name() string
}

// ChannelFactory 渠道工厂函数类型
type ChannelFactory func() Channel

// 渠道注册表
var (
	channelRegistry = make(map[string]ChannelFactory)
	registryMutex   sync.RWMutex
)

// EmailChannel 邮件通知渠道
type EmailChannel struct{}

// NewEmailChannel 创建一个新的EmailChannel实例
// @return 新创建的EmailChannel实例
func NewEmailChannel() *EmailChannel {
	return &EmailChannel{}
}

// Send 发送邮件通知
// @param notification 待发送的通知
// @return 发送过程中的错误
func (c *EmailChannel) Send(notification *model.Notification) error {
	metrics := metrics.GetMetrics()
	metrics.IncrementTotal()
	metrics.IncrementChannelTotal("email")

	err := retry.Do(func() error {
		// 实际实现中这里会调用邮件服务
		logger.Info("Sending email to user %s with content: %s", notification.UserID, notification.Content)
		// 模拟发送成功
		// 如果需要模拟失败，可以返回 errors.New("email send failed")
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementFailed()
		metrics.IncrementChannelFailed("email")
		return err
	}

	metrics.IncrementSuccessful()
	metrics.IncrementChannelSuccessful("email")
	return nil
}

// Name 获取渠道名称
func (c *EmailChannel) Name() string {
	return "email"
}

// InAppChannel 应用内通知渠道
type InAppChannel struct{}

// NewInAppChannel 创建一个新的InAppChannel实例
// @return 新创建的InAppChannel实例
func NewInAppChannel() *InAppChannel {
	return &InAppChannel{}
}

// Send 发送应用内通知
// @param notification 待发送的通知
// @return 发送过程中的错误
func (c *InAppChannel) Send(notification *model.Notification) error {
	metrics := metrics.GetMetrics()
	metrics.IncrementTotal()
	metrics.IncrementChannelTotal("inapp")

	err := retry.Do(func() error {
		// 实际实现中这里会调用应用内通知服务
		logger.Info("Sending in-app notification to user %s with content: %s", notification.UserID, notification.Content)
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementFailed()
		metrics.IncrementChannelFailed("inapp")
		return err
	}

	metrics.IncrementSuccessful()
	metrics.IncrementChannelSuccessful("inapp")
	return nil
}

// Name 获取渠道名称
func (c *InAppChannel) Name() string {
	return "inapp"
}

// SMSChannel 短信通知渠道
type SMSChannel struct{}

// NewSMSChannel 创建一个新的SMSChannel实例
// @return 新创建的SMSChannel实例
func NewSMSChannel() *SMSChannel {
	return &SMSChannel{}
}

// Send 发送短信通知
// @param notification 待发送的通知
// @return 发送过程中的错误
func (c *SMSChannel) Send(notification *model.Notification) error {
	metrics := metrics.GetMetrics()
	metrics.IncrementTotal()
	metrics.IncrementChannelTotal("sms")

	err := retry.Do(func() error {
		// 实际实现中这里会调用短信服务
		logger.Info("Sending SMS to user %s with content: %s", notification.UserID, notification.Content)
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementFailed()
		metrics.IncrementChannelFailed("sms")
		return err
	}

	metrics.IncrementSuccessful()
	metrics.IncrementChannelSuccessful("sms")
	return nil
}

// Name 获取渠道名称
func (c *SMSChannel) Name() string {
	return "sms"
}

// SocialMediaChannel 社交媒体通知渠道
type SocialMediaChannel struct{}

// NewSocialMediaChannel 创建一个新的SocialMediaChannel实例
// @return 新创建的SocialMediaChannel实例
func NewSocialMediaChannel() *SocialMediaChannel {
	return &SocialMediaChannel{}
}

// Send 发送社交媒体通知
// @param notification 待发送的通知
// @return 发送过程中的错误
func (c *SocialMediaChannel) Send(notification *model.Notification) error {
	metrics := metrics.GetMetrics()
	metrics.IncrementTotal()
	metrics.IncrementChannelTotal("social")

	err := retry.Do(func() error {
		// 实际实现中这里会调用社交媒体服务
		logger.Info("Sending social media notification to user %s with content: %s", notification.UserID, notification.Content)
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementFailed()
		metrics.IncrementChannelFailed("social")
		return err
	}

	metrics.IncrementSuccessful()
	metrics.IncrementChannelSuccessful("social")
	return nil
}

// Name 获取渠道名称
func (c *SocialMediaChannel) Name() string {
	return "social"
}

// RegisterChannel 注册渠道工厂函数
// @param name 渠道名称
// @param factory 渠道工厂函数
func RegisterChannel(name string, factory ChannelFactory) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	channelRegistry[name] = factory
	logger.Info("Channel registered: %s", name)
}

// GetChannel 获取渠道实例
// @param name 渠道名称
// @return 渠道实例
func GetChannel(name string) (Channel, error) {
	registryMutex.RLock()
	factory, ok := channelRegistry[name]
	registryMutex.RUnlock()

	if !ok {
		err := errors.ChannelError("Channel not found", fmt.Sprintf("Channel %s not registered", name), nil)
		logger.Error("Failed to get channel: %v", err)
		return nil, err
	}

	return factory(), nil
}

// GetAllChannels 获取所有注册的渠道
// @return 渠道名称列表
func GetAllChannels() []string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	channels := make([]string, 0, len(channelRegistry))
	for name := range channelRegistry {
		channels = append(channels, name)
	}

	return channels
}

// 初始化函数，注册默认渠道
func init() {
	// 注册默认渠道
	RegisterChannel("email", func() Channel { return NewEmailChannel() })
	RegisterChannel("inapp", func() Channel { return NewInAppChannel() })
	RegisterChannel("sms", func() Channel { return NewSMSChannel() })
	RegisterChannel("social", func() Channel { return NewSocialMediaChannel() })
}
