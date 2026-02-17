package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Server     ServerConfig
	Slack      SlackConfig
	Docker     DockerConfig
	SelfHosted bool
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// SlackConfig holds Slack integration settings.
type SlackConfig struct {
	BotToken      string
	SigningSecret string
	AppToken      string
}

// DockerConfig holds container runtime settings.
type DockerConfig struct {
	Host         string
	ImageDefault string
	CPULimit     string
	MemLimit     string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	dbPort, err := getEnvInt("AIRA_DB_PORT", 5432)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	dbMaxConns, err := getEnvInt("AIRA_DB_MAX_CONNS", 25)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	redisDB, err := getEnvInt("AIRA_REDIS_DB", 0)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	accessTTL, err := getEnvDuration("AIRA_JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	refreshTTL, err := getEnvDuration("AIRA_JWT_REFRESH_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	readTimeout, err := getEnvDuration("AIRA_SERVER_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	writeTimeout, err := getEnvDuration("AIRA_SERVER_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	selfHosted, err := getEnvBool("AIRA_SELF_HOSTED", false)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("AIRA_DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("AIRA_DB_USER", "aira"),
			Password: getEnv("AIRA_DB_PASSWORD", "aira"),
			DBName:   getEnv("AIRA_DB_NAME", "aira_dev"),
			SSLMode:  getEnv("AIRA_DB_SSLMODE", "disable"),
			MaxConns: dbMaxConns,
		},
		Redis: RedisConfig{
			Addr:     getEnv("AIRA_REDIS_ADDR", "localhost:6379"),
			Password: getEnv("AIRA_REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		JWT: JWTConfig{
			Secret:     getEnv("AIRA_JWT_SECRET", "change-me-in-production"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		Server: ServerConfig{
			Addr:         getEnv("AIRA_SERVER_ADDR", ":8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
		Slack: SlackConfig{
			BotToken:      getEnv("AIRA_SLACK_BOT_TOKEN", ""),
			SigningSecret: getEnv("AIRA_SLACK_SIGNING_SECRET", ""),
			AppToken:      getEnv("AIRA_SLACK_APP_TOKEN", ""),
		},
		Docker: DockerConfig{
			Host:         getEnv("AIRA_DOCKER_HOST", "unix:///var/run/docker.sock"),
			ImageDefault: getEnv("AIRA_DOCKER_IMAGE_DEFAULT", "ghcr.io/gosuda/aira-agent:latest"),
			CPULimit:     getEnv("AIRA_DOCKER_CPU_LIMIT", "2"),
			MemLimit:     getEnv("AIRA_DOCKER_MEM_LIMIT", "2g"),
		},
		SelfHosted: selfHosted,
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parsing %s=%q as int: %w", key, v, err)
	}
	return n, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("parsing %s=%q as bool: %w", key, v, err)
	}
	return b, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parsing %s=%q as duration: %w", key, v, err)
	}
	return d, nil
}
