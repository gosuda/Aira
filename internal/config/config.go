package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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
	Password string //nolint:gosec // G117: DB connection config
	DBName   string
	SSLMode  string
	MaxConns int
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string //nolint:gosec // G117: Redis connection config
	DB       int
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret     string //nolint:gosec // G117: JWT signing secret config
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	CORSOrigins  []string
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

// Load reads configuration from environment variables.
// Defaults are safe for local development only. In production,
// sensitive values (JWT secret, DB password) must be set explicitly.
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

	corsOrigins := getEnvList("AIRA_CORS_ORIGINS", []string{"http://localhost:5173"})

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("AIRA_DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("AIRA_DB_USER", "aira"),
			Password: getEnv("AIRA_DB_PASSWORD", ""),
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
			Secret:     getEnv("AIRA_JWT_SECRET", ""),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		Server: ServerConfig{
			Addr:         getEnv("AIRA_SERVER_ADDR", ":8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			CORSOrigins:  corsOrigins,
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

	err = cfg.validate()
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	return cfg, nil
}

// validate checks required fields and value bounds.
func (c *Config) validate() error {
	// JWT secret is required (no insecure default).
	if c.JWT.Secret == "" {
		return errors.New("AIRA_JWT_SECRET is required")
	}
	if len(c.JWT.Secret) < 32 {
		return errors.New("AIRA_JWT_SECRET must be at least 32 characters")
	}

	// DB SSL mode warning for non-self-hosted deployments.
	if c.Database.SSLMode == "disable" && !c.SelfHosted {
		log.Warn().Msg("AIRA_DB_SSLMODE=disable is insecure for production; set to 'require' or 'verify-full'")
	}

	// Bounds checks.
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("AIRA_DB_PORT must be 1-65535, got %d", c.Database.Port)
	}
	if c.Database.MaxConns < 1 {
		return fmt.Errorf("AIRA_DB_MAX_CONNS must be >= 1, got %d", c.Database.MaxConns)
	}
	if c.JWT.AccessTTL <= 0 {
		return fmt.Errorf("AIRA_JWT_ACCESS_TTL must be positive, got %s", c.JWT.AccessTTL)
	}
	if c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("AIRA_JWT_REFRESH_TTL must be positive, got %s", c.JWT.RefreshTTL)
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("AIRA_SERVER_READ_TIMEOUT must be positive, got %s", c.Server.ReadTimeout)
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("AIRA_SERVER_WRITE_TIMEOUT must be positive, got %s", c.Server.WriteTimeout)
	}

	return nil
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

func getEnvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
