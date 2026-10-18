package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultDBPort    = "5432"
	defaultDBSSLMode = "disable"
)

var validSSLModes = map[string]bool{
	"disable":     true,
	"allow":       true,
	"prefer":      true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Name     string
	SSLMode  string
}

// LoadDBConfig reads the database settings from the environment.
// DB_USER, DB_PASSWORD, DB_HOST and DB_NAME are required; DB_PORT and DB_SSLMODE are optional.
func LoadDBConfig() (DBConfig, error) {
	cfg := DBConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Host:     strings.TrimSpace(os.Getenv("DB_HOST")),
		Port:     strings.TrimSpace(os.Getenv("DB_PORT")),
		Name:     strings.TrimSpace(os.Getenv("DB_NAME")),
		SSLMode:  strings.TrimSpace(os.Getenv("DB_SSLMODE")),
	}

	var missing []string
	required := []struct {
		key   string
		value string
	}{
		{"DB_USER", cfg.User},
		{"DB_PASSWORD", cfg.Password},
		{"DB_HOST", cfg.Host},
		{"DB_NAME", cfg.Name},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.key)
		}
	}
	if len(missing) > 0 {
		return DBConfig{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if cfg.Port == "" {
		cfg.Port = defaultDBPort
	}
	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		return DBConfig{}, fmt.Errorf("invalid DB_PORT %q: must be a number between 1 and 65535", cfg.Port)
	}

	if cfg.SSLMode == "" {
		cfg.SSLMode = defaultDBSSLMode
	}
	if !validSSLModes[cfg.SSLMode] {
		return DBConfig{}, fmt.Errorf("invalid DB_SSLMODE %q", cfg.SSLMode)
	}

	return cfg, nil
}

// DSN builds the postgres connection string, escaping credentials so passwords
// containing characters like '@', ':' or '/' don't break the URL.
func (c DBConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}
	return u.String()
}
