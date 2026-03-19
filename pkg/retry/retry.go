package retry

import (
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
	Backoff     time.Duration
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 3,
		Delay:       1 * time.Second,
		Backoff:     2 * time.Second,
	}
}

func Do(fn func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var err error
	for i := 0; i < config.MaxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < config.MaxAttempts-1 {
			time.Sleep(config.Delay + time.Duration(i)*config.Backoff)
		}
	}

	return err
}
