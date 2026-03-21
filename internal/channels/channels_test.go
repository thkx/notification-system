package channels

import (
	"fmt"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/thkx/notification-system/pkg/model"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

func TestRegisterChannel(t *testing.T) {
	// 注册一个自定义渠道
	customChannelName := "custom"
	RegisterChannel(customChannelName, func() Channel {
		return &CustomChannel{}
	})

	// 验证渠道是否注册成功
	channel, err := GetChannel(customChannelName)
	if err != nil {
		t.Errorf("GetChannel() error = %v, want nil", err)
	}
	if channel == nil {
		t.Errorf("GetChannel() channel = nil, want non-nil")
	}
	if channel.Name() != customChannelName {
		t.Errorf("Channel.Name() = %v, want %v", channel.Name(), customChannelName)
	}
}

func TestGetAllChannels(t *testing.T) {
	channels := GetAllChannels()
	if len(channels) == 0 {
		t.Errorf("GetAllChannels() returned empty list, want at least default channels")
	}

	// 验证默认渠道是否存在
	expectedChannels := []string{"email", "inapp", "sms", "social"}
	for _, expected := range expectedChannels {
		found := false
		for _, actual := range channels {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected channel %s not found", expected)
		}
	}
}

func TestGetChannelNotFound(t *testing.T) {
	channel, err := GetChannel("non-existent-channel")
	if err == nil {
		t.Errorf("GetChannel() error = nil, want non-nil")
	}
	if channel != nil {
		t.Errorf("GetChannel() channel = %v, want nil", channel)
	}
}

func TestSendGridSendEmailRequiresConfiguration(t *testing.T) {
	t.Setenv("SENDGRID_API_KEY", "")
	t.Setenv("SENDGRID_FROM_EMAIL", "")

	err := sendGridSendEmail(&model.Notification{
		ID:      "notif-1",
		UserID:  "user@example.com",
		Type:    "order",
		Content: "content",
	})
	if err == nil {
		t.Fatal("expected missing SendGrid configuration to fail")
	}
}

func TestSendGridSendEmailUsesSDK(t *testing.T) {
	original := sendGridSend
	t.Cleanup(func() {
		sendGridSend = original
	})

	t.Setenv("SENDGRID_API_KEY", "key")
	t.Setenv("SENDGRID_FROM_EMAIL", "from@example.com")
	t.Setenv("SENDGRID_FROM_NAME", "Notifier")

	called := false
	sendGridSend = func(apiKey string, message *mail.SGMailV3) (*rest.Response, error) {
		called = true
		if apiKey != "key" {
			t.Fatalf("unexpected api key %q", apiKey)
		}
		if got := message.Personalizations[0].To[0].Address; got != "user@example.com" {
			t.Fatalf("unexpected to email %q", got)
		}
		return &rest.Response{StatusCode: 202}, nil
	}

	err := sendGridSendEmail(&model.Notification{
		ID:      "notif-1",
		UserID:  "user@example.com",
		Type:    "order",
		Content: "content",
	})
	if err != nil {
		t.Fatalf("expected sendgrid send to succeed, got %v", err)
	}
	if !called {
		t.Fatal("expected SendGrid SDK to be called")
	}
}

func TestTwilioSendSMSRequiresConfiguration(t *testing.T) {
	t.Setenv("TWILIO_ACCOUNT_SID", "")
	t.Setenv("TWILIO_AUTH_TOKEN", "")
	t.Setenv("TWILIO_FROM_NUMBER", "")

	err := twilioSendSMS(&model.Notification{
		ID:      "notif-1",
		UserID:  "+15555550123",
		Content: "content",
	})
	if err == nil {
		t.Fatal("expected missing Twilio configuration to fail")
	}
}

func TestTwilioSendSMSUsesSDK(t *testing.T) {
	original := twilioCreateMessage
	t.Cleanup(func() {
		twilioCreateMessage = original
	})

	t.Setenv("TWILIO_ACCOUNT_SID", "sid")
	t.Setenv("TWILIO_AUTH_TOKEN", "token")
	t.Setenv("TWILIO_FROM_NUMBER", "+15550000000")

	called := false
	twilioCreateMessage = func(accountSID string, authToken string, params *openapi.CreateMessageParams) error {
		called = true
		if accountSID != "sid" || authToken != "token" {
			return fmt.Errorf("unexpected credentials")
		}
		if params.To == nil || *params.To != "+15555550123" {
			t.Fatalf("unexpected to number")
		}
		if params.From == nil || *params.From != "+15550000000" {
			t.Fatalf("unexpected from number")
		}
		return nil
	}

	err := twilioSendSMS(&model.Notification{
		ID:      "notif-1",
		UserID:  "+15555550123",
		Content: "content",
	})
	if err != nil {
		t.Fatalf("expected twilio send to succeed, got %v", err)
	}
	if !called {
		t.Fatal("expected Twilio SDK to be called")
	}
}

// CustomChannel 自定义渠道实现
type CustomChannel struct{}

func (c *CustomChannel) Send(notification *model.Notification) error {
	return nil
}

func (c *CustomChannel) Name() string {
	return "custom"
}
