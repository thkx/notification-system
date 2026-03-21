package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestServerConfigValidation 测试服务器配置验证
func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{"Valid port", ServerConfig{Port: 8080}, false},
		{"Invalid port 0", ServerConfig{Port: 0}, true},
		{"Invalid port negative", ServerConfig{Port: -1}, true},
		{"Invalid port too large", ServerConfig{Port: 65536}, true},
		{"Valid max port", ServerConfig{Port: 65535}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRouterConfigValidation 测试路由器配置验证
func TestRouterConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  RouterConfig
		wantErr bool
	}{
		{"Valid config", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100}, false},
		{"Invalid buffer size 0", RouterConfig{BufferSize: 0, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"Invalid worker count 0", RouterConfig{BufferSize: 1000, WorkerCount: 0, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"Invalid max retries negative", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: -1, RetryDelayMs: 100}, true},
		{"Invalid retry delay negative", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: -1}, true},
		{"Buffer size too large", RouterConfig{BufferSize: 100001, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"Worker count too large", RouterConfig{BufferSize: 1000, WorkerCount: 1001, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"Max retries too large", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 11, RetryDelayMs: 100}, true},
		{"Retry delay too large", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 60001}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMetricsConfigValidation 测试性能监控配置验证
func TestMetricsConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  MetricsConfig
		wantErr bool
	}{
		{"Valid config", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, false},
		{"Invalid failure rate negative", MetricsConfig{MaxFailureRate: -0.1, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, true},
		{"Invalid failure rate > 1", MetricsConfig{MaxFailureRate: 1.1, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, true},
		{"Invalid queue utilization negative", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: -0.1, MaxProcessingTime: 5000}, true},
		{"Invalid queue utilization > 1", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 1.1, MaxProcessingTime: 5000}, true},
		{"Invalid processing time 0", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 0}, true},
		{"Invalid processing time negative", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: -1}, true},
		{"Processing time too large", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 300001}, true},
		{"Valid rate 0", MetricsConfig{MaxFailureRate: 0, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, false},
		{"Valid rate 1", MetricsConfig{MaxFailureRate: 1, MaxQueueUtilization: 1, MaxProcessingTime: 5000}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDistributionConfigValidation 测试分发配置验证
func TestDistributionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  DistributionConfig
		wantErr bool
	}{
		{"Valid config", DistributionConfig{DeduplicationTTL: 60}, false},
		{"Invalid TTL 0", DistributionConfig{DeduplicationTTL: 0}, true},
		{"Invalid TTL negative", DistributionConfig{DeduplicationTTL: -1}, true},
		{"TTL too large", DistributionConfig{DeduplicationTTL: 86401}, true},
		{"Valid max TTL", DistributionConfig{DeduplicationTTL: 86400}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigValidation 测试全局Config验证
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			"Valid config",
			Config{
				Server:       ServerConfig{Port: 8080},
				Router:       RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100},
				Metrics:      MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000},
				Distribution: DistributionConfig{DeduplicationTTL: 60},
			},
			false,
		},
		{
			"Invalid server",
			Config{
				Server:       ServerConfig{Port: 0},
				Router:       RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100},
				Metrics:      MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000},
				Distribution: DistributionConfig{DeduplicationTTL: 60},
			},
			true,
		},
		{
			"Invalid router",
			Config{
				Server:       ServerConfig{Port: 8080},
				Router:       RouterConfig{BufferSize: 0, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100},
				Metrics:      MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000},
				Distribution: DistributionConfig{DeduplicationTTL: 60},
				Store:        StoreConfig{Type: "memory"},
			},
			true,
		},
		{
			"Invalid store type",
			Config{
				Server:       ServerConfig{Port: 8080},
				Router:       RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100},
				Metrics:      MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000},
				Distribution: DistributionConfig{DeduplicationTTL: 60},
				Store:        StoreConfig{Type: "unknown"},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConfigLoadsWithoutDeadlock(t *testing.T) {
	originalConfig := globalConfig
	globalConfig = nil
	t.Cleanup(func() {
		globalConfig = originalConfig
	})

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.test.json")
	content := `{
		"server": {"port": 8080},
		"router": {"bufferSize": 10, "workerCount": 1, "maxRetries": 1, "retryDelayMs": 10},
		"channels": {
			"email": {"enabled": true},
			"sms": {"enabled": true},
			"inapp": {"enabled": true},
			"social_media": {"enabled": true}
		},
		"metrics": {"maxFailureRate": 0.2, "maxQueueUtilization": 0.8, "maxProcessingTime": 5000},
		"distribution": {"deduplicationTTL": 60},
		"store": {"type": "memory"},
		"environment": "test"
	}`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWd)
	})

	t.Setenv("NOTIFICATION_ENV", "test")

	done := make(chan *Config, 1)
	go func() {
		done <- GetConfig()
	}()

	select {
	case cfg := <-done:
		if cfg == nil {
			t.Fatal("expected config to be loaded")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("GetConfig/LoadConfig path appears blocked")
	}
}
