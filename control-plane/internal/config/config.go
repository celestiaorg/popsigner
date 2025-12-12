// Package config provides configuration loading for the Control Plane API.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	OpenBao  OpenBaoConfig  `mapstructure:"openbao"`
	Auth     AuthConfig     `mapstructure:"auth"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Host         string        `mapstructure:"host"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	Environment  string        `mapstructure:"environment"` // dev, staging, prod
}

// DatabaseConfig holds PostgreSQL configuration.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN returns the PostgreSQL connection string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr returns the Redis address string.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// OpenBaoConfig holds OpenBao configuration.
type OpenBaoConfig struct {
	Address       string `mapstructure:"address"`
	Token         string `mapstructure:"token"`
	Namespace     string `mapstructure:"namespace"`
	Secp256k1Path string `mapstructure:"secp256k1_path"`
}

// AuthConfig holds authentication configuration.
// POPSigner uses OAuth-only authentication (no email/password).
type AuthConfig struct {
	JWTSecret         string        `mapstructure:"jwt_secret"`
	JWTExpiry         time.Duration `mapstructure:"jwt_expiry"`
	SessionExpiry     time.Duration `mapstructure:"session_expiry"`
	OAuthGitHubID     string        `mapstructure:"oauth_github_id"`
	OAuthGitHubSecret string        `mapstructure:"oauth_github_secret"`
	OAuthGoogleID     string        `mapstructure:"oauth_google_id"`
	OAuthGoogleSecret string        `mapstructure:"oauth_google_secret"`
	OAuthCallbackURL  string        `mapstructure:"oauth_callback_url"`
	DashboardURL      string        `mapstructure:"dashboard_url"`
}

// Load reads configuration from files and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/popsigner")

	// Enable environment variable override
	// Internal: Uses BANHBAO_ prefix for operator compatibility
	// The operator uses banhbaoring.io CRDs - full migration to popsigner.com CRDs is v1.1
	v.SetEnvPrefix("BANHBAO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Explicitly bind OAuth environment variables (nested struct issue with viper)
	v.BindEnv("auth.oauth_github_id", "BANHBAO_AUTH_OAUTH_GITHUB_ID")
	v.BindEnv("auth.oauth_github_secret", "BANHBAO_AUTH_OAUTH_GITHUB_SECRET")
	v.BindEnv("auth.oauth_google_id", "BANHBAO_AUTH_OAUTH_GOOGLE_ID")
	v.BindEnv("auth.oauth_google_secret", "BANHBAO_AUTH_OAUTH_GOOGLE_SECRET")
	v.BindEnv("auth.oauth_callback_url", "BANHBAO_AUTH_OAUTH_CALLBACK_URL")

	// Explicitly bind OpenBao environment variables
	v.BindEnv("openbao.address", "BANHBAO_OPENBAO_ADDRESS")
	v.BindEnv("openbao.token", "BANHBAO_OPENBAO_TOKEN")
	v.BindEnv("openbao.namespace", "BANHBAO_OPENBAO_NAMESPACE")
	v.BindEnv("openbao.secp256k1_path", "BANHBAO_OPENBAO_SECP256K1_PATH")

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK, we use defaults and env vars
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults configures default values for all settings.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.environment", "dev")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "popsigner")
	v.SetDefault("database.password", "popsigner")
	v.SetDefault("database.database", "popsigner")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	// OpenBao defaults
	v.SetDefault("openbao.address", "http://localhost:8200")
	v.SetDefault("openbao.namespace", "")
	v.SetDefault("openbao.secp256k1_path", "secp256k1") // Use secp256k1 plugin

	// Auth defaults (OAuth-only, no email/password)
	v.SetDefault("auth.jwt_expiry", "24h")
	v.SetDefault("auth.session_expiry", "168h") // 7 days
	v.SetDefault("auth.oauth_callback_url", "http://localhost:8080")
	v.SetDefault("auth.dashboard_url", "http://localhost:3000")
}

