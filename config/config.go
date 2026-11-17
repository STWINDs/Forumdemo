package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Mode    string `mapstructure:"mode"`
	Version string `mapstructure:"version"`
	Port    int    `mapstructure:"port"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Mode     string   `mapstructure:"mode"`
	Addrs    []string `mapstructure:"addrs"`
	Host     string   `mapstructure:"host"`
	Port     int      `mapstructure:"port"`
	Password string   `mapstructure:"password"`
	DB       int      `mapstructure:"db"`
	PoolSize int      `mapstructure:"pool_size"`
}

type KafkaConfig struct {
	Address string `mapstructure:"address"`
	Topic   string `mapstructure:"topic"`
}

type MinioConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	BucketName      string `mapstructure:"bucket_name"`
}

// RateLimitConfig separates the maximum burst from the sustained refill
// rate. Both values are per application process; a strict cluster-wide limit
// belongs at the gateway or in a shared store.
type RateLimitConfig struct {
	Enabled bool    `mapstructure:"enabled"`
	Burst   int64   `mapstructure:"burst"`
	RPS     float64 `mapstructure:"rps"`
}

type DirectMessageOutboxConfig struct {
	BatchSize    int           `mapstructure:"batch_size"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	RetryDelay   time.Duration `mapstructure:"retry_delay"`
	LeaseTimeout time.Duration `mapstructure:"lease_timeout"`
	Workers      int           `mapstructure:"workers"`
}

type Config struct {
	App                 *AppConfig                 `mapstructure:"app"`
	MySQL               *MySQLConfig               `mapstructure:"mysql"`
	Redis               *RedisConfig               `mapstructure:"redis"`
	Kafka               *KafkaConfig               `mapstructure:"kafka"`
	Minio               *MinioConfig               `mapstructure:"minio"`
	RateLimit           *RateLimitConfig           `mapstructure:"rate_limit"`
	DirectMessageOutbox *DirectMessageOutboxConfig `mapstructure:"direct_message_outbox"`
}

var Conf = new(Config)

func Init() (err error) {
	viper.SetConfigFile("config/config.yaml")
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.burst", 200)
	viper.SetDefault("rate_limit.rps", 100)
	viper.SetDefault("direct_message_outbox.batch_size", 100)
	viper.SetDefault("direct_message_outbox.poll_interval", time.Second)
	viper.SetDefault("direct_message_outbox.retry_delay", time.Minute)
	viper.SetDefault("direct_message_outbox.lease_timeout", time.Minute)
	viper.SetDefault("direct_message_outbox.workers", 1)
	// 读取环境变量
	viper.AutomaticEnv()
	// 环境变量前缀（可选，如果设置了，环境变量需要加上前缀，如 FORUM_MYSQL_HOST）
	// viper.SetEnvPrefix("FORUM")
	// 将环境变量中的 _ 替换为 .，以便匹配 mapstructure
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.ReadInConfig()
	if err != nil {
		fmt.Printf("viper.ReadInConfig failed, err:%v\n", err)
		return
	}
	if err := viper.Unmarshal(Conf); err != nil {
		fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
		return err
	}
	// Viper/mapstructure does not consistently split a comma-separated env
	// value into []string. Handle REDIS_ADDRS explicitly so Docker and
	// orchestrators can inject Cluster seed nodes without rewriting YAML.
	if raw, ok := os.LookupEnv("REDIS_ADDRS"); ok {
		Conf.Redis.Addrs = nil
		for _, addr := range strings.Split(raw, ",") {
			if addr = strings.TrimSpace(addr); addr != "" {
				Conf.Redis.Addrs = append(Conf.Redis.Addrs, addr)
			}
		}
	}
	return validateRuntimeConfig(Conf)
}

func validateRuntimeConfig(cfg *Config) error {
	if cfg.RateLimit == nil {
		return fmt.Errorf("rate_limit config is required")
	}
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.Burst <= 0 {
			return fmt.Errorf("rate_limit.burst must be greater than zero")
		}
		if cfg.RateLimit.RPS <= 0 {
			return fmt.Errorf("rate_limit.rps must be greater than zero")
		}
	}
	if cfg.DirectMessageOutbox == nil {
		return fmt.Errorf("direct_message_outbox config is required")
	}
	if cfg.DirectMessageOutbox.BatchSize <= 0 {
		return fmt.Errorf("direct_message_outbox.batch_size must be greater than zero")
	}
	if cfg.DirectMessageOutbox.PollInterval <= 0 {
		return fmt.Errorf("direct_message_outbox.poll_interval must be greater than zero")
	}
	if cfg.DirectMessageOutbox.RetryDelay <= 0 {
		return fmt.Errorf("direct_message_outbox.retry_delay must be greater than zero")
	}
	if cfg.DirectMessageOutbox.LeaseTimeout <= 0 {
		return fmt.Errorf("direct_message_outbox.lease_timeout must be greater than zero")
	}
	if cfg.DirectMessageOutbox.Workers <= 0 {
		return fmt.Errorf("direct_message_outbox.workers must be greater than zero")
	}
	return nil
}
