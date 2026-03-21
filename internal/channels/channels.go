package channels

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/thkx/notification-system/pkg/errors"
	"github.com/thkx/notification-system/pkg/logger"
	"github.com/thkx/notification-system/pkg/metrics"
	"github.com/thkx/notification-system/pkg/model"
	"github.com/thkx/notification-system/pkg/retry"
	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
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

	sendGridSend = func(apiKey string, message *mail.SGMailV3) (*rest.Response, error) {
		return sendgrid.NewSendClient(apiKey).Send(message)
	}
	twilioCreateMessage = func(accountSID string, authToken string, params *openapi.CreateMessageParams) error {
		client := twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: accountSID,
			Password: authToken,
		})
		_, err := client.Api.CreateMessage(params)
		return err
	}
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
	metrics.IncrementChannelTotal("email")

	var err error
	provider := normalizedProvider(os.Getenv("EMAIL_PROVIDER"))
	if provider == "sendgrid" {
		err = sendGridSendEmail(notification)
	} else {
		err = retry.Do(func() error {
			// 本地模拟发送
			logger.Info("Sending email (default provider) to user %s with content: %s", notification.UserID, notification.Content)
			return nil
		}, retry.DefaultRetryConfig())
	}

	if err != nil {
		metrics.IncrementChannelFailed("email")
		return err
	}

	metrics.IncrementChannelSuccessful("email")
	return nil
}

// Name 获取渠道名称
func (c *EmailChannel) Name() string {
	return "email"
}

func sendGridSendEmail(notification *model.Notification) error {
	apiKey := strings.TrimSpace(os.Getenv("SENDGRID_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("SENDGRID_API_KEY is required when EMAIL_PROVIDER=sendgrid")
	}

	fromEmail := strings.TrimSpace(os.Getenv("SENDGRID_FROM_EMAIL"))
	if fromEmail == "" {
		return fmt.Errorf("SENDGRID_FROM_EMAIL is required when EMAIL_PROVIDER=sendgrid")
	}
	fromName := strings.TrimSpace(os.Getenv("SENDGRID_FROM_NAME"))
	toEmail := strings.TrimSpace(notification.UserID)
	if toEmail == "" {
		return fmt.Errorf("notification UserID must be a recipient email when EMAIL_PROVIDER=sendgrid")
	}

	message := mail.NewSingleEmail(
		mail.NewEmail(fromName, fromEmail),
		buildEmailSubject(notification),
		mail.NewEmail("", toEmail),
		notification.Content,
		notification.Content,
	)

	response, err := sendGridSend(apiKey, message)
	if err != nil {
		return fmt.Errorf("sendgrid send failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sendgrid send failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(response.Body))
	}

	logger.Info("SendGrid sent email to %s with status %d", toEmail, response.StatusCode)
	return nil
}

func twilioSendSMS(notification *model.Notification) error {
	accountSid := strings.TrimSpace(os.Getenv("TWILIO_ACCOUNT_SID"))
	authToken := strings.TrimSpace(os.Getenv("TWILIO_AUTH_TOKEN"))
	if accountSid == "" || authToken == "" {
		return fmt.Errorf("TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN are required when SMS_PROVIDER=twilio")
	}

	fromNumber := strings.TrimSpace(os.Getenv("TWILIO_FROM_NUMBER"))
	if fromNumber == "" {
		return fmt.Errorf("TWILIO_FROM_NUMBER is required when SMS_PROVIDER=twilio")
	}
	toNumber := strings.TrimSpace(notification.UserID)
	if toNumber == "" {
		return fmt.Errorf("notification UserID must be a recipient phone number when SMS_PROVIDER=twilio")
	}

	params := &openapi.CreateMessageParams{}
	params.SetTo(toNumber)
	params.SetFrom(fromNumber)
	params.SetBody(notification.Content)

	if err := twilioCreateMessage(accountSid, authToken, params); err != nil {
		return fmt.Errorf("twilio send failed: %w", err)
	}

	logger.Info("Twilio sent SMS to %s", toNumber)
	return nil
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
	metrics.IncrementChannelTotal("inapp")

	err := retry.Do(func() error {
		// 实际实现中这里会调用应用内通知服务
		logger.Info("Sending in-app notification to user %s with content: %s", notification.UserID, notification.Content)
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementChannelFailed("inapp")
		return err
	}

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
	metrics.IncrementChannelTotal("sms")

	var err error
	provider := normalizedProvider(os.Getenv("SMS_PROVIDER"))
	if provider == "twilio" {
		err = twilioSendSMS(notification)
	} else {
		err = retry.Do(func() error {
			// 默认本地模拟发送
			logger.Info("Sending SMS (default provider) to user %s with content: %s", notification.UserID, notification.Content)
			return nil
		}, retry.DefaultRetryConfig())
	}

	if err != nil {
		metrics.IncrementChannelFailed("sms")
		return err
	}

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
	metrics.IncrementChannelTotal("social")

	err := retry.Do(func() error {
		// 实际实现中这里会调用社交媒体服务
		logger.Info("Sending social media notification to user %s with content: %s", notification.UserID, notification.Content)
		return nil
	}, retry.DefaultRetryConfig())

	if err != nil {
		metrics.IncrementChannelFailed("social")
		return err
	}

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

func normalizedProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return "memory"
	}
	return provider
}

func buildEmailSubject(notification *model.Notification) string {
	if notification == nil {
		return "Notification"
	}
	if strings.TrimSpace(notification.Type) == "" {
		return fmt.Sprintf("Notification %s", notification.ID)
	}
	return fmt.Sprintf("[%s] Notification %s", strings.ToUpper(notification.Type), notification.ID)
}
