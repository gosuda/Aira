package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setVal   *string // nil = don't set; pointer to distinguish "" from unset
		fallback string
		want     string
	}{
		{name: "returns fallback when unset", key: "AIRA_TEST_GETENV_UNSET", setVal: nil, fallback: "default", want: "default"},
		{name: "returns env value when set", key: "AIRA_TEST_GETENV_SET", setVal: strPtr("custom"), fallback: "default", want: "custom"},
		{name: "returns fallback when empty string", key: "AIRA_TEST_GETENV_EMPTY", setVal: strPtr(""), fallback: "default", want: "default"},
		{name: "preserves whitespace", key: "AIRA_TEST_GETENV_WS", setVal: strPtr("  spaced  "), fallback: "x", want: "  spaced  "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setVal != nil {
				t.Setenv(tc.key, *tc.setVal)
			}

			got := getEnv(tc.key, tc.fallback)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setVal   *string
		fallback int
		want     int
		wantErr  bool
	}{
		{name: "returns fallback when unset", key: "AIRA_TEST_INT_UNSET", setVal: nil, fallback: 42, want: 42},
		{name: "parses valid int", key: "AIRA_TEST_INT_VALID", setVal: strPtr("8080"), fallback: 0, want: 8080},
		{name: "parses negative int", key: "AIRA_TEST_INT_NEG", setVal: strPtr("-1"), fallback: 0, want: -1},
		{name: "parses zero", key: "AIRA_TEST_INT_ZERO", setVal: strPtr("0"), fallback: 99, want: 0},
		{name: "returns fallback for empty string", key: "AIRA_TEST_INT_EMPTY", setVal: strPtr(""), fallback: 25, want: 25},
		{name: "errors on non-numeric", key: "AIRA_TEST_INT_NAN", setVal: strPtr("abc"), fallback: 0, wantErr: true},
		{name: "errors on float", key: "AIRA_TEST_INT_FLOAT", setVal: strPtr("3.14"), fallback: 0, wantErr: true},
		{name: "errors on hex", key: "AIRA_TEST_INT_HEX", setVal: strPtr("0xFF"), fallback: 0, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setVal != nil {
				t.Setenv(tc.key, *tc.setVal)
			}

			got, err := getEnvInt(tc.key, tc.fallback)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.key)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setVal   *string
		fallback bool
		want     bool
		wantErr  bool
	}{
		{name: "returns fallback when unset", key: "AIRA_TEST_BOOL_UNSET", setVal: nil, fallback: false, want: false},
		{name: "fallback true when unset", key: "AIRA_TEST_BOOL_UNSETTRUE", setVal: nil, fallback: true, want: true},
		{name: "parses true", key: "AIRA_TEST_BOOL_TRUE", setVal: strPtr("true"), fallback: false, want: true},
		{name: "parses false", key: "AIRA_TEST_BOOL_FALSE", setVal: strPtr("false"), fallback: true, want: false},
		{name: "parses 1", key: "AIRA_TEST_BOOL_ONE", setVal: strPtr("1"), fallback: false, want: true},
		{name: "parses 0", key: "AIRA_TEST_BOOL_ZERO", setVal: strPtr("0"), fallback: true, want: false},
		{name: "parses TRUE uppercase", key: "AIRA_TEST_BOOL_UPPER", setVal: strPtr("TRUE"), fallback: false, want: true},
		{name: "parses t", key: "AIRA_TEST_BOOL_T", setVal: strPtr("t"), fallback: false, want: true},
		{name: "errors on invalid", key: "AIRA_TEST_BOOL_INV", setVal: strPtr("yes"), fallback: false, wantErr: true},
		{name: "errors on numeric non-bool", key: "AIRA_TEST_BOOL_NUM", setVal: strPtr("2"), fallback: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setVal != nil {
				t.Setenv(tc.key, *tc.setVal)
			}

			got, err := getEnvBool(tc.key, tc.fallback)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.key)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setVal   *string
		fallback time.Duration
		want     time.Duration
		wantErr  bool
	}{
		{name: "returns fallback when unset", key: "AIRA_TEST_DUR_UNSET", setVal: nil, fallback: 5 * time.Second, want: 5 * time.Second},
		{name: "parses seconds", key: "AIRA_TEST_DUR_SEC", setVal: strPtr("30s"), fallback: 0, want: 30 * time.Second},
		{name: "parses minutes", key: "AIRA_TEST_DUR_MIN", setVal: strPtr("15m"), fallback: 0, want: 15 * time.Minute},
		{name: "parses hours", key: "AIRA_TEST_DUR_HR", setVal: strPtr("2h"), fallback: 0, want: 2 * time.Hour},
		{name: "parses composite", key: "AIRA_TEST_DUR_COMP", setVal: strPtr("1h30m"), fallback: 0, want: 90 * time.Minute},
		{name: "parses nanosecond", key: "AIRA_TEST_DUR_NS", setVal: strPtr("1ns"), fallback: 0, want: time.Nanosecond},
		{name: "parses zero", key: "AIRA_TEST_DUR_ZERO", setVal: strPtr("0s"), fallback: 5 * time.Second, want: 0},
		{name: "errors on invalid", key: "AIRA_TEST_DUR_INV", setVal: strPtr("notaduration"), fallback: 0, wantErr: true},
		{name: "errors on bare number", key: "AIRA_TEST_DUR_BARE", setVal: strPtr("30"), fallback: 0, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setVal != nil {
				t.Setenv(tc.key, *tc.setVal)
			}

			got, err := getEnvDuration(tc.key, tc.fallback)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.key)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Load() error cases
// ---------------------------------------------------------------------------

func TestLoad_MissingJWTSecret(t *testing.T) {
	// All defaults apply; JWT secret is empty => must fail.
	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "AIRA_JWT_SECRET")
}

func TestLoad_InvalidEnvVars(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		envVal string
		errMsg string
	}{
		// DB_PORT parse errors
		{name: "DB_PORT not a number", envKey: "AIRA_DB_PORT", envVal: "abc", errMsg: "AIRA_DB_PORT"},
		{name: "DB_PORT float", envKey: "AIRA_DB_PORT", envVal: "3.14", errMsg: "AIRA_DB_PORT"},

		// DB_PORT validation errors (parses fine, fails bounds)
		{name: "DB_PORT zero", envKey: "AIRA_DB_PORT", envVal: "0", errMsg: "AIRA_DB_PORT"},
		{name: "DB_PORT negative", envKey: "AIRA_DB_PORT", envVal: "-1", errMsg: "AIRA_DB_PORT"},
		{name: "DB_PORT too high", envKey: "AIRA_DB_PORT", envVal: "65536", errMsg: "AIRA_DB_PORT"},

		// DB_MAX_CONNS
		{name: "DB_MAX_CONNS zero", envKey: "AIRA_DB_MAX_CONNS", envVal: "0", errMsg: "AIRA_DB_MAX_CONNS"},
		{name: "DB_MAX_CONNS negative", envKey: "AIRA_DB_MAX_CONNS", envVal: "-5", errMsg: "AIRA_DB_MAX_CONNS"},
		{name: "DB_MAX_CONNS not a number", envKey: "AIRA_DB_MAX_CONNS", envVal: "many", errMsg: "AIRA_DB_MAX_CONNS"},

		// JWT durations
		{name: "JWT_ACCESS_TTL invalid", envKey: "AIRA_JWT_ACCESS_TTL", envVal: "badval", errMsg: "AIRA_JWT_ACCESS_TTL"},
		{name: "JWT_REFRESH_TTL invalid", envKey: "AIRA_JWT_REFRESH_TTL", envVal: "badval", errMsg: "AIRA_JWT_REFRESH_TTL"},
		{name: "JWT_ACCESS_TTL zero", envKey: "AIRA_JWT_ACCESS_TTL", envVal: "0s", errMsg: "AIRA_JWT_ACCESS_TTL"},
		{name: "JWT_REFRESH_TTL zero", envKey: "AIRA_JWT_REFRESH_TTL", envVal: "0s", errMsg: "AIRA_JWT_REFRESH_TTL"},
		{name: "JWT_ACCESS_TTL negative", envKey: "AIRA_JWT_ACCESS_TTL", envVal: "-5m", errMsg: "AIRA_JWT_ACCESS_TTL"},
		{name: "JWT_REFRESH_TTL negative", envKey: "AIRA_JWT_REFRESH_TTL", envVal: "-1h", errMsg: "AIRA_JWT_REFRESH_TTL"},

		// Server timeouts
		{name: "SERVER_READ_TIMEOUT invalid", envKey: "AIRA_SERVER_READ_TIMEOUT", envVal: "notduration", errMsg: "AIRA_SERVER_READ_TIMEOUT"},
		{name: "SERVER_WRITE_TIMEOUT invalid", envKey: "AIRA_SERVER_WRITE_TIMEOUT", envVal: "notduration", errMsg: "AIRA_SERVER_WRITE_TIMEOUT"},
		{name: "SERVER_READ_TIMEOUT zero", envKey: "AIRA_SERVER_READ_TIMEOUT", envVal: "0s", errMsg: "AIRA_SERVER_READ_TIMEOUT"},
		{name: "SERVER_WRITE_TIMEOUT zero", envKey: "AIRA_SERVER_WRITE_TIMEOUT", envVal: "0s", errMsg: "AIRA_SERVER_WRITE_TIMEOUT"},

		// Redis DB
		{name: "REDIS_DB not a number", envKey: "AIRA_REDIS_DB", envVal: "abc", errMsg: "AIRA_REDIS_DB"},

		// Self-hosted
		{name: "SELF_HOSTED not a bool", envKey: "AIRA_SELF_HOSTED", envVal: "yes", errMsg: "AIRA_SELF_HOSTED"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Always set JWT secret so failures are from the var under test.
			t.Setenv("AIRA_JWT_SECRET", "test-secret-for-error-cases-32ch!")
			t.Setenv(tc.envKey, tc.envVal)

			cfg, err := Load()
			require.Error(t, err, "expected error for %s=%q", tc.envKey, tc.envVal)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// Load() edge cases -- boundary values
// ---------------------------------------------------------------------------

func TestLoad_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		envs     map[string]string
		assertFn func(t *testing.T, cfg *Config)
	}{
		{
			name: "port min boundary 1",
			envs: map[string]string{
				"AIRA_JWT_SECRET": "test-secret-that-is-at-least-32ch",
				"AIRA_DB_PORT":    "1",
			},
			assertFn: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 1, cfg.Database.Port)
			},
		},
		{
			name: "port max boundary 65535",
			envs: map[string]string{
				"AIRA_JWT_SECRET": "test-secret-that-is-at-least-32ch",
				"AIRA_DB_PORT":    "65535",
			},
			assertFn: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 65535, cfg.Database.Port)
			},
		},
		{
			name: "MaxConns min boundary 1",
			envs: map[string]string{
				"AIRA_JWT_SECRET":   "test-secret-that-is-at-least-32ch",
				"AIRA_DB_MAX_CONNS": "1",
			},
			assertFn: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 1, cfg.Database.MaxConns)
			},
		},
		{
			name: "duration 1ns is valid",
			envs: map[string]string{
				"AIRA_JWT_SECRET":           "test-secret-that-is-at-least-32ch",
				"AIRA_JWT_ACCESS_TTL":       "1ns",
				"AIRA_JWT_REFRESH_TTL":      "1ns",
				"AIRA_SERVER_READ_TIMEOUT":  "1ns",
				"AIRA_SERVER_WRITE_TIMEOUT": "1ns",
			},
			assertFn: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, time.Nanosecond, cfg.JWT.AccessTTL)
				assert.Equal(t, time.Nanosecond, cfg.JWT.RefreshTTL)
				assert.Equal(t, time.Nanosecond, cfg.Server.ReadTimeout)
				assert.Equal(t, time.Nanosecond, cfg.Server.WriteTimeout)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.NoError(t, err)
			require.NotNil(t, cfg)
			tc.assertFn(t, cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// Load() happy paths
// ---------------------------------------------------------------------------

func TestLoad_Defaults(t *testing.T) {
	// Only the required JWT secret is set; everything else uses defaults.
	t.Setenv("AIRA_JWT_SECRET", "my-dev-secret-at-least-32-chars!!")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Database defaults.
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "aira", cfg.Database.User)
	assert.Empty(t, cfg.Database.Password)
	assert.Equal(t, "aira_dev", cfg.Database.DBName)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, 25, cfg.Database.MaxConns)

	// Redis defaults.
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Empty(t, cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)

	// JWT defaults.
	assert.Equal(t, "my-dev-secret-at-least-32-chars!!", cfg.JWT.Secret)
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 7*24*time.Hour, cfg.JWT.RefreshTTL)

	// Server defaults.
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)

	// Slack defaults.
	assert.Empty(t, cfg.Slack.BotToken)
	assert.Empty(t, cfg.Slack.SigningSecret)
	assert.Empty(t, cfg.Slack.AppToken)

	// Docker defaults.
	assert.Equal(t, "unix:///var/run/docker.sock", cfg.Docker.Host)
	assert.Equal(t, "ghcr.io/gosuda/aira-agent:latest", cfg.Docker.ImageDefault)
	assert.Equal(t, "2", cfg.Docker.CPULimit)
	assert.Equal(t, "2g", cfg.Docker.MemLimit)

	// Self-hosted default.
	assert.False(t, cfg.SelfHosted)
}

func TestLoad_AllCustomValues(t *testing.T) {
	envs := map[string]string{
		// Database
		"AIRA_DB_HOST":      "db.prod.internal",
		"AIRA_DB_PORT":      "5433",
		"AIRA_DB_USER":      "prod_user",
		"AIRA_DB_PASSWORD":  "s3cret!",
		"AIRA_DB_NAME":      "aira_prod",
		"AIRA_DB_SSLMODE":   "require",
		"AIRA_DB_MAX_CONNS": "50",
		// Redis
		"AIRA_REDIS_ADDR":     "redis.prod:6380",
		"AIRA_REDIS_PASSWORD": "redis-pass",
		"AIRA_REDIS_DB":       "3",
		// JWT
		"AIRA_JWT_SECRET":      "prod-jwt-secret-256-bits-long!!!",
		"AIRA_JWT_ACCESS_TTL":  "30m",
		"AIRA_JWT_REFRESH_TTL": "72h",
		// Server
		"AIRA_SERVER_ADDR":          ":9090",
		"AIRA_SERVER_READ_TIMEOUT":  "5s",
		"AIRA_SERVER_WRITE_TIMEOUT": "15s",
		// Slack
		"AIRA_SLACK_BOT_TOKEN":      "xoxb-test",
		"AIRA_SLACK_SIGNING_SECRET": "slack-sign",
		"AIRA_SLACK_APP_TOKEN":      "xapp-test",
		// Docker
		"AIRA_DOCKER_HOST":          "tcp://docker:2375",
		"AIRA_DOCKER_IMAGE_DEFAULT": "myregistry/agent:v2",
		"AIRA_DOCKER_CPU_LIMIT":     "4",
		"AIRA_DOCKER_MEM_LIMIT":     "8g",
		// Self-hosted
		"AIRA_SELF_HOSTED": "true",
	}

	for k, v := range envs {
		t.Setenv(k, v)
	}

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Database
	assert.Equal(t, "db.prod.internal", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "prod_user", cfg.Database.User)
	assert.Equal(t, "s3cret!", cfg.Database.Password)
	assert.Equal(t, "aira_prod", cfg.Database.DBName)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, 50, cfg.Database.MaxConns)

	// Redis
	assert.Equal(t, "redis.prod:6380", cfg.Redis.Addr)
	assert.Equal(t, "redis-pass", cfg.Redis.Password)
	assert.Equal(t, 3, cfg.Redis.DB)

	// JWT
	assert.Equal(t, "prod-jwt-secret-256-bits-long!!!", cfg.JWT.Secret)
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 72*time.Hour, cfg.JWT.RefreshTTL)

	// Server
	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.WriteTimeout)

	// Slack
	assert.Equal(t, "xoxb-test", cfg.Slack.BotToken)
	assert.Equal(t, "slack-sign", cfg.Slack.SigningSecret)
	assert.Equal(t, "xapp-test", cfg.Slack.AppToken)

	// Docker
	assert.Equal(t, "tcp://docker:2375", cfg.Docker.Host)
	assert.Equal(t, "myregistry/agent:v2", cfg.Docker.ImageDefault)
	assert.Equal(t, "4", cfg.Docker.CPULimit)
	assert.Equal(t, "8g", cfg.Docker.MemLimit)

	// Self-hosted
	assert.True(t, cfg.SelfHosted)
}

// ---------------------------------------------------------------------------
// DSN() output format
// ---------------------------------------------------------------------------

func TestDatabaseConfig_DSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "default dev values",
			cfg: DatabaseConfig{
				Host: "localhost", Port: 5432, User: "aira",
				Password: "", DBName: "aira_dev", SSLMode: "disable",
			},
			want: "host=localhost port=5432 user=aira password= dbname=aira_dev sslmode=disable",
		},
		{
			name: "production values",
			cfg: DatabaseConfig{
				Host: "db.prod", Port: 5433, User: "admin",
				Password: "p@ss!", DBName: "aira_prod", SSLMode: "require",
			},
			want: "host=db.prod port=5433 user=admin password=p@ss! dbname=aira_prod sslmode=require",
		},
		{
			name: "special characters in password",
			cfg: DatabaseConfig{
				Host: "h", Port: 1, User: "u",
				Password: "p=a&b c", DBName: "d", SSLMode: "s",
			},
			want: "host=h port=1 user=u password=p=a&b c dbname=d sslmode=s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.cfg.DSN())
		})
	}
}

// ---------------------------------------------------------------------------
// DSN() integration with Load()
// ---------------------------------------------------------------------------

func TestLoad_DSN_Integration(t *testing.T) {
	t.Setenv("AIRA_JWT_SECRET", "test-secret-that-is-at-least-32ch")
	t.Setenv("AIRA_DB_HOST", "myhost")
	t.Setenv("AIRA_DB_PORT", "5433")
	t.Setenv("AIRA_DB_USER", "myuser")
	t.Setenv("AIRA_DB_PASSWORD", "mypass")
	t.Setenv("AIRA_DB_NAME", "mydb")
	t.Setenv("AIRA_DB_SSLMODE", "verify-full")

	cfg, err := Load()
	require.NoError(t, err)

	want := "host=myhost port=5433 user=myuser password=mypass dbname=mydb sslmode=verify-full"
	assert.Equal(t, want, cfg.Database.DSN())
}

// ---------------------------------------------------------------------------
// validate() direct tests
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	t.Parallel()

	// validBase returns a Config that passes validation.
	validBase := func() *Config {
		return &Config{
			Database: DatabaseConfig{Port: 5432, MaxConns: 25},
			JWT: JWTConfig{
				Secret:     "test-secret-that-is-at-least-32ch",
				AccessTTL:  15 * time.Minute,
				RefreshTTL: 7 * 24 * time.Hour,
			},
			Server: ServerConfig{
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 30 * time.Second,
			},
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, validBase().validate())
	})

	t.Run("empty JWT secret fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.Secret = ""
		assert.ErrorContains(t, c.validate(), "AIRA_JWT_SECRET")
	})

	t.Run("JWT secret too short fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.Secret = "only-31-characters-long-secret!"
		assert.ErrorContains(t, c.validate(), "AIRA_JWT_SECRET")
	})

	t.Run("JWT secret exactly 32 chars passes", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.Secret = "exactly-32-characters-long-sec!!"
		assert.NoError(t, c.validate())
	})

	t.Run("port 0 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.Port = 0
		assert.ErrorContains(t, c.validate(), "AIRA_DB_PORT")
	})

	t.Run("port 65536 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.Port = 65536
		assert.ErrorContains(t, c.validate(), "AIRA_DB_PORT")
	})

	t.Run("port 1 passes", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.Port = 1
		assert.NoError(t, c.validate())
	})

	t.Run("port 65535 passes", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.Port = 65535
		assert.NoError(t, c.validate())
	})

	t.Run("MaxConns 0 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.MaxConns = 0
		assert.ErrorContains(t, c.validate(), "AIRA_DB_MAX_CONNS")
	})

	t.Run("MaxConns negative fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.MaxConns = -10
		assert.ErrorContains(t, c.validate(), "AIRA_DB_MAX_CONNS")
	})

	t.Run("MaxConns 1 passes", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Database.MaxConns = 1
		assert.NoError(t, c.validate())
	})

	t.Run("AccessTTL 0 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.AccessTTL = 0
		assert.ErrorContains(t, c.validate(), "AIRA_JWT_ACCESS_TTL")
	})

	t.Run("AccessTTL negative fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.AccessTTL = -time.Minute
		assert.ErrorContains(t, c.validate(), "AIRA_JWT_ACCESS_TTL")
	})

	t.Run("AccessTTL 1ns passes", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.AccessTTL = time.Nanosecond
		assert.NoError(t, c.validate())
	})

	t.Run("RefreshTTL negative fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.JWT.RefreshTTL = -time.Minute
		assert.ErrorContains(t, c.validate(), "AIRA_JWT_REFRESH_TTL")
	})

	t.Run("ReadTimeout 0 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Server.ReadTimeout = 0
		assert.ErrorContains(t, c.validate(), "AIRA_SERVER_READ_TIMEOUT")
	})

	t.Run("ReadTimeout negative fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Server.ReadTimeout = -time.Second
		assert.ErrorContains(t, c.validate(), "AIRA_SERVER_READ_TIMEOUT")
	})

	t.Run("WriteTimeout 0 fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Server.WriteTimeout = 0
		assert.ErrorContains(t, c.validate(), "AIRA_SERVER_WRITE_TIMEOUT")
	})

	t.Run("WriteTimeout negative fails", func(t *testing.T) {
		t.Parallel()
		c := validBase()
		c.Server.WriteTimeout = -time.Second
		assert.ErrorContains(t, c.validate(), "AIRA_SERVER_WRITE_TIMEOUT")
	})
}

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

func strPtr(s string) *string {
	return &s
}
