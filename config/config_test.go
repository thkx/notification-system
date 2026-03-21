package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validServerConfig() ServerConfig {
	return ServerConfig{
		Port:           8080,
		ReadTimeoutMs:  5000,
		WriteTimeoutMs: 10000,
		IdleTimeoutMs:  60000,
		MaxBodyBytes:   1 << 20,
	}
}

func validConfig() Config {
	return Config{
		Server:       validServerConfig(),
		Security:     SecurityConfig{},
		Router:       RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100},
		Metrics:      MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000},
		Distribution: DistributionConfig{DeduplicationTTL: 60},
		Store:        StoreConfig{Type: "memory"},
	}
}

func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{"valid config", validServerConfig(), false},
		{"invalid port", ServerConfig{Port: 0, ReadTimeoutMs: 1, WriteTimeoutMs: 1, IdleTimeoutMs: 1, MaxBodyBytes: 1}, true},
		{"invalid read timeout", ServerConfig{Port: 8080, ReadTimeoutMs: 0, WriteTimeoutMs: 1, IdleTimeoutMs: 1, MaxBodyBytes: 1}, true},
		{"invalid write timeout", ServerConfig{Port: 8080, ReadTimeoutMs: 1, WriteTimeoutMs: 0, IdleTimeoutMs: 1, MaxBodyBytes: 1}, true},
		{"invalid idle timeout", ServerConfig{Port: 8080, ReadTimeoutMs: 1, WriteTimeoutMs: 1, IdleTimeoutMs: 0, MaxBodyBytes: 1}, true},
		{"invalid body limit", ServerConfig{Port: 8080, ReadTimeoutMs: 1, WriteTimeoutMs: 1, IdleTimeoutMs: 1, MaxBodyBytes: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurityConfigValidation(t *testing.T) {
	invalid := SecurityConfig{RequireAPIKey: true}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected missing API key to fail validation")
	}

	valid := SecurityConfig{RequireAPIKey: true, APIKey: "secret"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid security config, got %v", err)
	}
}

func TestRouterConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  RouterConfig
		wantErr bool
	}{
		{"valid config", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100}, false},
		{"invalid buffer size", RouterConfig{BufferSize: 0, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"invalid worker count", RouterConfig{BufferSize: 1000, WorkerCount: 0, MaxRetries: 3, RetryDelayMs: 100}, true},
		{"invalid max retries", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: -1, RetryDelayMs: 100}, true},
		{"invalid retry delay", RouterConfig{BufferSize: 1000, WorkerCount: 3, MaxRetries: 3, RetryDelayMs: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetricsConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  MetricsConfig
		wantErr bool
	}{
		{"valid config", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, false},
		{"invalid failure rate", MetricsConfig{MaxFailureRate: -0.1, MaxQueueUtilization: 0.8, MaxProcessingTime: 5000}, true},
		{"invalid queue utilization", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 1.1, MaxProcessingTime: 5000}, true},
		{"invalid processing time", MetricsConfig{MaxFailureRate: 0.2, MaxQueueUtilization: 0.8, MaxProcessingTime: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDistributionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  DistributionConfig
		wantErr bool
	}{
		{"valid config", DistributionConfig{DeduplicationTTL: 60}, false},
		{"invalid ttl", DistributionConfig{DeduplicationTTL: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	cfg.Security = SecurityConfig{RequireAPIKey: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid security config to fail")
	}
}

func TestLoadConfigFromFileUsesDefaultsForMissingFields(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.json")
	content := `{
		"server": {"port": 8080},
		"security": {"requireApiKey": false},
		"router": {"bufferSize": 10, "workerCount": 1, "maxRetries": 1, "retryDelayMs": 10},
		"metrics": {"maxFailureRate": 0.2, "maxQueueUtilization": 0.8, "maxProcessingTime": 5000},
		"distribution": {"deduplicationTTL": 60},
		"store": {"type": "memory"},
		"environment": "test"
	}`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() error = %v", err)
	}

	if cfg.Server.ReadTimeoutMs == 0 || cfg.Server.MaxBodyBytes == 0 {
		t.Fatal("expected default server values to be applied")
	}
}

func TestLoadConfigFromFileRejectsInvalidSecurityConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.json")
	content := `{
		"server": {"port": 8080},
		"security": {"requireApiKey": true},
		"router": {"bufferSize": 10, "workerCount": 1, "maxRetries": 1, "retryDelayMs": 10},
		"metrics": {"maxFailureRate": 0.2, "maxQueueUtilization": 0.8, "maxProcessingTime": 5000},
		"distribution": {"deduplicationTTL": 60},
		"store": {"type": "memory"},
		"environment": "production"
	}`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if _, err := LoadConfigFromFile(configFile); err == nil {
		t.Fatal("expected invalid security config to fail")
	}
}

func TestLoadConfigAppliesEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.test.json")
	content := `{
		"server": {"port": 8080},
		"security": {"requireApiKey": true, "apiKey": "config-secret"},
		"router": {"bufferSize": 10, "workerCount": 1, "maxRetries": 1, "retryDelayMs": 10},
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
	t.Setenv("NOTIFICATION_API_KEY", "env-secret")
	t.Setenv("NOTIFICATION_ALLOWED_ORIGINS", "http://localhost:3000, https://example.com")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Security.APIKey != "env-secret" {
		t.Fatalf("expected env API key override, got %q", cfg.Security.APIKey)
	}
	if len(cfg.Server.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d", len(cfg.Server.AllowedOrigins))
	}
}
