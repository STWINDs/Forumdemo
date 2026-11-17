package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func validRuntimeConfig() *Config {
	return &Config{
		RateLimit: &RateLimitConfig{Enabled: true, Burst: 10, RPS: 5},
		DirectMessageOutbox: &DirectMessageOutboxConfig{
			BatchSize: 100, PollInterval: time.Second, RetryDelay: time.Minute,
			LeaseTimeout: time.Minute, Workers: 1,
		},
	}
}

func TestConfigYAMLParsesPerformanceSettings(t *testing.T) {
	// Runtime config.yaml is gitignored and may contain deployment credentials.
	const fixture = `
rate_limit:
  enabled: true
  burst: 200
  rps: 100
direct_message_outbox:
  batch_size: 100
  poll_interval: 1s
  retry_delay: 1m
  lease_timeout: 1m
  workers: 1
`
	reader := viper.New()
	reader.SetConfigType("yaml")
	require.NoError(t, reader.ReadConfig(strings.NewReader(fixture)))
	var cfg Config
	require.NoError(t, reader.Unmarshal(&cfg))
	require.NoError(t, validateRuntimeConfig(&cfg))
	require.Equal(t, int64(200), cfg.RateLimit.Burst)
	require.Equal(t, float64(100), cfg.RateLimit.RPS)
	require.Equal(t, time.Second, cfg.DirectMessageOutbox.PollInterval)
}

func TestValidateRuntimeConfig(t *testing.T) {
	require.NoError(t, validateRuntimeConfig(validRuntimeConfig()))

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"zero burst", func(c *Config) { c.RateLimit.Burst = 0 }},
		{"zero rps", func(c *Config) { c.RateLimit.RPS = 0 }},
		{"zero batch", func(c *Config) { c.DirectMessageOutbox.BatchSize = 0 }},
		{"zero poll", func(c *Config) { c.DirectMessageOutbox.PollInterval = 0 }},
		{"zero retry", func(c *Config) { c.DirectMessageOutbox.RetryDelay = 0 }},
		{"zero lease", func(c *Config) { c.DirectMessageOutbox.LeaseTimeout = 0 }},
		{"zero workers", func(c *Config) { c.DirectMessageOutbox.Workers = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validRuntimeConfig()
			tt.mutate(cfg)
			require.Error(t, validateRuntimeConfig(cfg))
		})
	}
}

func TestDisabledRateLimitAllowsZeroValues(t *testing.T) {
	cfg := validRuntimeConfig()
	cfg.RateLimit = &RateLimitConfig{Enabled: false}
	require.NoError(t, validateRuntimeConfig(cfg))
}
