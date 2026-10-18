package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort      = "8080"
	defaultMode      = "release"
	defaultDBPort    = "5432"
	defaultDBSSLMode = "disable"
)

var validModes = map[string]bool{
	"debug":   true,
	"release": true,
	"test":    true,
}

var validSSLModes = map[string]bool{
	"disable":     true,
	"allow":       true,
	"prefer":      true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

type Config struct {
	Server ServerConfig
	DB     DBConfig
}

type ServerConfig struct {
	Port    string
	BaseURL string // always ends with "/"
	Mode    string // gin mode: debug, release or test
}

// Load reads the full application config from the environment and reports
// every invalid setting at once, so a bad deploy shows all problems together.
func Load() (Config, error) {
	server, serverErr := LoadServerConfig()
	db, dbErr := LoadDBConfig()
	if err := errors.Join(serverErr, dbErr); err != nil {
		return Config{}, err
	}
	return Config{Server: server, DB: db}, nil
}

// LoadServerConfig reads the HTTP server settings from the environment. All are optional:
// PORT defaults to 8080, APP_MODE to release and BASE_URL to http://localhost:<PORT>/.
// APP_MODE is used instead of GIN_MODE because gin panics on an invalid GIN_MODE
// during package init, before this validation could report it.
func LoadServerConfig() (ServerConfig, error) {
	cfg := ServerConfig{
		Port:    strings.TrimSpace(os.Getenv("PORT")),
		BaseURL: strings.TrimSpace(os.Getenv("BASE_URL")),
		Mode:    strings.TrimSpace(os.Getenv("APP_MODE")),
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if cfg.Mode == "" {
		cfg.Mode = defaultMode
	}

	var errs []error
	portErr := validatePort("PORT", cfg.Port)
	if portErr != nil {
		errs = append(errs, portErr)
	}
	// only derive the default BASE_URL from a valid port, to avoid a second, confusing error
	if cfg.BaseURL == "" && portErr == nil {
		cfg.BaseURL = "http://localhost:" + cfg.Port + "/"
	}
	if cfg.BaseURL != "" {
		baseURL, err := normalizeBaseURL(cfg.BaseURL)
		if err != nil {
			errs = append(errs, err)
		}
		cfg.BaseURL = baseURL
	}
	if !validModes[cfg.Mode] {
		errs = append(errs, fmt.Errorf("invalid APP_MODE %q: must be one of debug, release, test", cfg.Mode))
	}
	if len(errs) > 0 {
		return ServerConfig{}, errors.Join(errs...)
	}

	return cfg, nil
}

// Addr is the listen address for the HTTP server, e.g. ":8080".
func (s ServerConfig) Addr() string {
	return ":" + s.Port
}

// normalizeBaseURL checks that raw is an absolute http(s) URL with no credentials,
// query or fragment, and makes sure it ends with "/" so short codes can be appended.
func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("invalid BASE_URL %q: must be an absolute http or https URL", raw)
	}
	if u.User != nil {
		return "", fmt.Errorf("invalid BASE_URL %q: must not contain credentials", u.Redacted())
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("invalid BASE_URL %q: must not contain a query or fragment", raw)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		if u.RawPath != "" {
			u.RawPath += "/"
		}
	}
	return u.String(), nil
}

func validatePort(key, value string) error {
	if port, err := strconv.Atoi(value); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid %s %q: must be a number between 1 and 65535", key, value)
	}
	return nil
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
	if err := validatePort("DB_PORT", cfg.Port); err != nil {
		return DBConfig{}, err
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
