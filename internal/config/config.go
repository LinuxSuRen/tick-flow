// Package config loads the server configuration. Environment variables take
// precedence over the optional YAML file, which in turn overrides built-in
// defaults.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// SMTPConfig describes an outbound SMTP server. When Host is empty, mail
// notifications are disabled.
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

// Config is the full server configuration.
type Config struct {
	Addr       string     `yaml:"addr"`
	DBDSN      string     `yaml:"db_dsn"`
	JWTSecret  string     `yaml:"jwt_secret"`
	UploadDir  string     `yaml:"upload_dir"`
	LogLevel   string     `yaml:"log_level"`
	SMTP       SMTPConfig `yaml:"smtp"`
	UploadMax  int64      `yaml:"upload_max_bytes"`
	UploadExts []string   `yaml:"upload_ext_whitelist"`
}

// Defaults used when neither env nor YAML provides a value.
const (
	defaultAddr     = ":8080"
	defaultDSN      = "postgres://tickflow:tickflow@localhost:5432/tickflow?sslmode=disable"
	defaultUpload   = "data/uploads"
	defaultMaxBytes = 20 << 20 // 20MB
)

var defaultExts = []string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".pdf", ".zip", ".txt"}

// Load builds the configuration: defaults <- YAML file <- environment.
// configPath may be empty, in which case only defaults and env apply.
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		Addr:       defaultAddr,
		DBDSN:      defaultDSN,
		UploadDir:  defaultUpload,
		LogLevel:   "info",
		UploadMax:  defaultMaxBytes,
		UploadExts: defaultExts,
	}

	if configPath == "" {
		configPath = os.Getenv("TICKFLOW_CONFIG")
	}
	if configPath != "" {
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", configPath, err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", configPath, err)
		}
	}

	applyEnv(cfg)

	if cfg.JWTSecret == "" {
		// No persistent secret configured: generate an ephemeral one so the
		// server can still boot (docker run quick start). Tokens die with the
		// process; a warn log tells the operator to configure it.
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate ephemeral jwt secret: %w", err)
		}
		cfg.JWTSecret = hex.EncodeToString(buf)
		slog.Warn("jwt secret not configured, using an ephemeral random secret; set TICKFLOW_JWT_SECRET for stable sessions")
	}
	if cfg.SMTP.Port == 0 && cfg.SMTP.Host != "" {
		cfg.SMTP.Port = 587
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	setStr("TICKFLOW_ADDR", &cfg.Addr)
	setStr("TICKFLOW_DB_DSN", &cfg.DBDSN)
	setStr("TICKFLOW_JWT_SECRET", &cfg.JWTSecret)
	setStr("TICKFLOW_UPLOAD_DIR", &cfg.UploadDir)
	setStr("TICKFLOW_LOG_LEVEL", &cfg.LogLevel)
	if v := os.Getenv("TICKFLOW_UPLOAD_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.UploadMax = n
		}
	}
	if v := os.Getenv("TICKFLOW_UPLOAD_EXT_WHITELIST"); v != "" {
		exts := splitComma(v)
		if len(exts) > 0 {
			cfg.UploadExts = exts
		}
	}
	setStr("TICKFLOW_SMTP_HOST", &cfg.SMTP.Host)
	setInt("TICKFLOW_SMTP_PORT", &cfg.SMTP.Port)
	setStr("TICKFLOW_SMTP_USERNAME", &cfg.SMTP.Username)
	setStr("TICKFLOW_SMTP_PASSWORD", &cfg.SMTP.Password)
	setStr("TICKFLOW_SMTP_FROM", &cfg.SMTP.From)
}

func setStr(name string, dst *string) {
	if v, ok := os.LookupEnv(name); ok {
		*dst = v
	}
}

func setInt(name string, dst *int) {
	if v, ok := os.LookupEnv(name); ok {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

func splitComma(v string) []string {
	var out []string
	cur := ""
	for _, r := range v {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
