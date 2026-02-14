package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	NATS      NATSConfig      `mapstructure:"nats"`
	Valkey    ValkeyConfig    `mapstructure:"valkey"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

type ServerConfig struct {
	Port         int `mapstructure:"port"`
	ReadTimeout  int `mapstructure:"read_timeout"`
	WriteTimeout int `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

type NATSConfig struct {
	URL string `mapstructure:"url"`
}

type ValkeyConfig struct {
	Addr string `mapstructure:"addr"`
}

type TelemetryConfig struct {
	ServiceName string `mapstructure:"service_name"`
	TempoAddr   string `mapstructure:"tempo_addr"`
	Enabled     bool   `mapstructure:"enabled"`
}

// Load reads configuration from file and environment variables.
func Load(service string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 10)
	v.SetDefault("server.write_timeout", 10)
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "transit")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "bilbopass")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("valkey.addr", "localhost:6379")
	v.SetDefault("telemetry.service_name", service)
	v.SetDefault("telemetry.tempo_addr", "tempo:4317")
	v.SetDefault("telemetry.enabled", true)

	// Config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	_ = v.ReadInConfig() // OK if missing

	// Environment variables: BILBOPASS_DATABASE_HOST → database.host
	v.SetEnvPrefix("BILBOPASS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that required configuration fields are present and sane.
func (c *Config) Validate() error {
	var errs []string

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be 1-65535, got %d", c.Server.Port))
	}
	if c.Database.Host == "" {
		errs = append(errs, "database.host is required")
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		errs = append(errs, fmt.Sprintf("database.port must be 1-65535, got %d", c.Database.Port))
	}
	if c.Database.User == "" {
		errs = append(errs, "database.user is required")
	}
	if c.Database.DBName == "" {
		errs = append(errs, "database.dbname is required")
	}
	if c.NATS.URL == "" {
		errs = append(errs, "nats.url is required")
	}
	if c.Valkey.Addr == "" {
		errs = append(errs, "valkey.addr is required")
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, "server.read_timeout must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		errs = append(errs, "server.write_timeout must be positive")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
