package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server           ServerConfig           `yaml:"server"`
	Database         DatabaseConfig         `yaml:"database"`
	WeWork           WeWorkConfig           `yaml:"wework"`
	Keys             KeysConfig             `yaml:"keys"`
	Features         FeaturesConfig         `yaml:"features"`
	Auth             AuthConfig             `yaml:"auth"`
	Scheduler        SchedulerConfig        `yaml:"scheduler"`
	ContactScheduler ContactSchedulerConfig `yaml:"contact_scheduler"`
	Nightly          NightlyConfig          `yaml:"nightly"`
	Redis            RedisConfig            `yaml:"redis"`
	RateLimit        RateLimitConfig        `yaml:"rate_limit"`
}

type ServerConfig struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	DBName          string        `yaml:"dbname"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type WeWorkConfig struct {
	BaseURL            string `yaml:"base_url"`
	CorpID             string `yaml:"corpid"`
	Secret             string `yaml:"secret"`
	ContactSecret      string `yaml:"contact_secret"`
	SyncLimit          int    `yaml:"sync_limit"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type KeysConfig struct {
	StoragePath    string `yaml:"storage_path"`
	DefaultVersion string `yaml:"default_version"`
	EncryptKey     string `yaml:"-"` // 不从 yaml 读取，仅环境变量
}

type FeaturesConfig struct {
	IDs   []int          `yaml:"ids"`
	Names map[int]string `yaml:"names"`
}

type AuthConfig struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	JWTSecret string `yaml:"jwt_secret"`
}

type SchedulerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"` // "1h", "30m", "24h"
}

type ContactSchedulerConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Interval   string `yaml:"interval"`
	StartDelay string `yaml:"start_delay"`
}

type NightlyConfig struct {
	Enabled      bool `yaml:"enabled"`
	Hour         int  `yaml:"hour"`          // 执行小时，默认 1
	Minute       int  `yaml:"minute"`        // 执行分钟，默认 0
	LookbackDays int  `yaml:"lookback_days"` // 回溯天数，默认 1（昨天）
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Stream   string `yaml:"stream"` // stream name
}

type RateLimitConfig struct {
	Enabled        bool `yaml:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_minute"`
	Burst          int  `yaml:"burst"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=0",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return value == "true" || value == "1" || value == "yes"
	}
	return fallback
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file failed: %w", err)
	}

	cfg.Database.Host = getEnv("DB_HOST", cfg.Database.Host)
	cfg.Database.Port = getEnvInt("DB_PORT", cfg.Database.Port)
	cfg.Database.User = getEnv("DB_USER", cfg.Database.User)
	cfg.Database.Password = getEnv("DB_PASSWORD", cfg.Database.Password)
	cfg.Database.DBName = getEnv("DB_NAME", cfg.Database.DBName)

	cfg.WeWork.BaseURL = getEnv("WEWORK_BASE_URL", cfg.WeWork.BaseURL)
	cfg.WeWork.CorpID = getEnv("WEWORK_CORPID", cfg.WeWork.CorpID)
	cfg.WeWork.Secret = getEnv("WEWORK_SECRET", cfg.WeWork.Secret)
	cfg.WeWork.ContactSecret = getEnv("WEWORK_CONTACT_SECRET", cfg.WeWork.ContactSecret)
	if v := os.Getenv("WEWORK_INSECURE_SKIP_VERIFY"); v != "" {
		cfg.WeWork.InsecureSkipVerify = v == "true" || v == "1"
	}

	cfg.Auth.Username = getEnv("AUTH_USERNAME", cfg.Auth.Username)
	cfg.Auth.Password = getEnv("AUTH_PASSWORD", cfg.Auth.Password)
	cfg.Auth.JWTSecret = getEnv("JWT_SECRET", cfg.Auth.JWTSecret)

	cfg.Keys.EncryptKey = os.Getenv("KEY_ENCRYPT_KEY")

	cfg.Redis.Host = getEnv("REDIS_HOST", "localhost")
	cfg.Redis.Port = getEnvInt("REDIS_PORT", 6379)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB = getEnvInt("REDIS_DB", 0)
	cfg.Redis.Stream = getEnv("REDIS_STREAM", "wwlocal:sync:tasks")

	cfg.RateLimit.Enabled = getEnvBool("RATE_LIMIT_ENABLED", true)
	cfg.RateLimit.RequestsPerMin = getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 100)
	cfg.RateLimit.Burst = getEnvInt("RATE_LIMIT_BURST", 20)

	cfg.Scheduler.Enabled = getEnvBool("SCHEDULER_ENABLED", cfg.Scheduler.Enabled)
	cfg.Scheduler.Interval = getEnv("SCHEDULER_INTERVAL", cfg.Scheduler.Interval)

	cfg.ContactScheduler.Enabled = getEnvBool("CONTACT_SCHEDULER_ENABLED", cfg.ContactScheduler.Enabled)
	cfg.ContactScheduler.Interval = getEnv("CONTACT_SCHEDULER_INTERVAL", cfg.ContactScheduler.Interval)
	cfg.ContactScheduler.StartDelay = getEnv("CONTACT_SCHEDULER_START_DELAY", cfg.ContactScheduler.StartDelay)

	cfg.Nightly.Enabled = getEnvBool("NIGHTLY_ENABLED", cfg.Nightly.Enabled)
	if cfg.Nightly.Hour <= 0 {
		cfg.Nightly.Hour = 1
	}
	if cfg.Nightly.LookbackDays <= 0 {
		cfg.Nightly.LookbackDays = 1
	}

	// 校验必需配置
	var missing []string
	if cfg.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if cfg.WeWork.BaseURL == "" {
		missing = append(missing, "WEWORK_BASE_URL")
	}
	if cfg.WeWork.CorpID == "" {
		missing = append(missing, "WEWORK_CORPID")
	}
	if cfg.WeWork.Secret == "" {
		missing = append(missing, "WEWORK_SECRET")
	}
	if cfg.WeWork.ContactSecret == "" {
		missing = append(missing, "WEWORK_CONTACT_SECRET")
	}
	if cfg.Auth.Password == "" {
		missing = append(missing, "AUTH_PASSWORD")
	}
	if cfg.Auth.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required config (set via env vars): %v", missing)
	}

	// 校验 JWT Secret 不为常见占位符值
	badSecrets := []string{
		"change-this-to-a-random-string",
		"changeme", "change-me",
		"secret", "password", "jwt-secret",
		"your-secret-here", "your_jwt_secret",
	}
	for _, bad := range badSecrets {
		if cfg.Auth.JWTSecret == bad {
			return nil, fmt.Errorf("JWT_SECRET is set to placeholder value %q, please use a strong random string (at least 32 chars)", bad)
		}
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET is too short (%d chars), must be at least 32 characters for security", len(cfg.Auth.JWTSecret))
	}

	return &cfg, nil
}
