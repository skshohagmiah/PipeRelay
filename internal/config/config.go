package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Delivery  DeliveryConfig  `mapstructure:"delivery"`
	Dashboard DashboardConfig `mapstructure:"dashboard"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Retention RetentionConfig `mapstructure:"retention"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type StorageConfig struct {
	Driver string       `mapstructure:"driver"`
	SQLite SQLiteConfig `mapstructure:"sqlite"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type DeliveryConfig struct {
	Workers       int             `mapstructure:"workers"`
	Timeout       time.Duration   `mapstructure:"timeout"`
	MaxAttempts   int             `mapstructure:"max_attempts"`
	RetrySchedule []time.Duration `mapstructure:"retry_schedule"`
}

type DashboardConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Path          string `mapstructure:"path"`
	AdminPassword string `mapstructure:"admin_password"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type RetentionConfig struct {
	MessageTTL time.Duration `mapstructure:"message_ttl"`
	AttemptTTL time.Duration `mapstructure:"attempt_ttl"`
}

func Load(path string) (*Config, error) {
	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("piperelay")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/piperelay")
	}

	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("PIPERELAY")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)

	viper.SetDefault("storage.driver", "sqlite")
	viper.SetDefault("storage.sqlite.path", "./data/piperelay.db")

	viper.SetDefault("delivery.workers", 50)
	viper.SetDefault("delivery.timeout", 30*time.Second)
	viper.SetDefault("delivery.max_attempts", 8)
	viper.SetDefault("delivery.retry_schedule", []time.Duration{
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		8 * time.Hour,
		24 * time.Hour,
	})

	viper.SetDefault("dashboard.enabled", true)
	viper.SetDefault("dashboard.path", "/dashboard")

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	viper.SetDefault("retention.message_ttl", 30*24*time.Hour)
	viper.SetDefault("retention.attempt_ttl", 7*24*time.Hour)
}
