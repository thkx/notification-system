package channels

import (
	"testing"

	"github.com/thkx/notification-system/pkg/model"
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

// CustomChannel 自定义渠道实现
type CustomChannel struct{}

func (c *CustomChannel) Send(notification *model.Notification) error {
	return nil
}

func (c *CustomChannel) Name() string {
	return "custom"
}
