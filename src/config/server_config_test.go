package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serverEnvKeys = []string{"PORT", "BASE_URL", "APP_MODE", "REDIRECT_STATUS"}

// setServerEnv sets every server variable, using "" for keys not in env.
func setServerEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, key := range serverEnvKeys {
		t.Setenv(key, env[key])
	}
}

func TestLoadServerConfig_Defaults(t *testing.T) {
	setServerEnv(t, map[string]string{})

	cfg, err := LoadServerConfig()

	require.NoError(t, err)
	assert.Equal(t, ServerConfig{Port: "8080", BaseURL: "http://localhost:8080/", Mode: "release", RedirectStatus: 302}, cfg)
}

func TestLoadServerConfig_WhitespaceOnlyUsesDefaults(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "  ", "BASE_URL": "\t", "APP_MODE": " ", "REDIRECT_STATUS": " "})

	cfg, err := LoadServerConfig()

	require.NoError(t, err)
	assert.Equal(t, ServerConfig{Port: "8080", BaseURL: "http://localhost:8080/", Mode: "release", RedirectStatus: 302}, cfg)
}

func TestLoadServerConfig_DefaultBaseURLFollowsPort(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "9090"})

	cfg, err := LoadServerConfig()

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090/", cfg.BaseURL)
}

func TestLoadServerConfig_AllValuesSet(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "3000", "BASE_URL": "https://sho.rt", "APP_MODE": "debug", "REDIRECT_STATUS": "301"})

	cfg, err := LoadServerConfig()

	require.NoError(t, err)
	assert.Equal(t, ServerConfig{Port: "3000", BaseURL: "https://sho.rt/", Mode: "debug", RedirectStatus: 301}, cfg)
}

func TestLoadServerConfig_TrimsWhitespace(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": " 3000 ", "BASE_URL": " https://sho.rt/ ", "APP_MODE": " test\n", "REDIRECT_STATUS": " 307 "})

	cfg, err := LoadServerConfig()

	require.NoError(t, err)
	assert.Equal(t, ServerConfig{Port: "3000", BaseURL: "https://sho.rt/", Mode: "test", RedirectStatus: 307}, cfg)
}

func TestLoadServerConfig_RedirectStatus(t *testing.T) {
	for _, status := range []string{"301", "302", "307", "308"} {
		t.Run("valid "+status, func(t *testing.T) {
			setServerEnv(t, map[string]string{"REDIRECT_STATUS": status})

			cfg, err := LoadServerConfig()

			require.NoError(t, err)
			assert.Equal(t, status, strconv.Itoa(cfg.RedirectStatus))
		})
	}

	// an invalid value is not fatal: it logs a warning and falls back to 302
	for _, status := range []string{"200", "303", "404", "abc", "301.0", "-302", "3\n02 forged=line"} {
		t.Run("invalid "+status, func(t *testing.T) {
			var logs bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(original) })
			setServerEnv(t, map[string]string{"REDIRECT_STATUS": status})

			cfg, err := LoadServerConfig()

			require.NoError(t, err)
			assert.Equal(t, 302, cfg.RedirectStatus)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
			assert.Equal(t, "WARN", entry["level"])
			assert.Equal(t, "invalid REDIRECT_STATUS, using 302", entry["msg"])
			assert.Equal(t, status, entry["value"])
			assert.Equal(t, "301, 302, 307, 308", entry["allowed"])
		})
	}
}

func TestLoadServerConfig_RejectsBaseURLPathReservedForLookupAPI(t *testing.T) {
	for _, baseURL := range []string{"https://sho.rt/url/", "https://sho.rt/url"} {
		t.Run(baseURL, func(t *testing.T) {
			setServerEnv(t, map[string]string{"BASE_URL": baseURL})

			cfg, err := LoadServerConfig()

			assert.EqualError(t, err, `invalid BASE_URL "`+baseURL+`": the path /url/ is reserved for the lookup API`)
			assert.Equal(t, ServerConfig{}, cfg)
		})
	}
}

func TestLoadServerConfig_Mode(t *testing.T) {
	for _, mode := range []string{"debug", "release", "test"} {
		t.Run("valid "+mode, func(t *testing.T) {
			setServerEnv(t, map[string]string{"APP_MODE": mode})

			cfg, err := LoadServerConfig()

			require.NoError(t, err)
			assert.Equal(t, mode, cfg.Mode)
		})
	}

	for _, mode := range []string{"production", "DEBUG", "Release", "dev"} {
		t.Run("invalid "+mode, func(t *testing.T) {
			setServerEnv(t, map[string]string{"APP_MODE": mode})

			cfg, err := LoadServerConfig()

			assert.EqualError(t, err, `invalid APP_MODE "`+mode+`": must be one of debug, release, test`)
			assert.Equal(t, ServerConfig{}, cfg)
		})
	}
}

func TestLoadServerConfig_Port(t *testing.T) {
	tests := []struct {
		port    string
		wantErr bool
	}{
		{"1", false},
		{"80", false},
		{"8080", false},
		{"65535", false},
		{"0", true},
		{"-1", true},
		{"65536", true},
		{"abc", true},
		{"80a", true},
		{":8080", true},
		{"8080.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			setServerEnv(t, map[string]string{"PORT": tt.port, "BASE_URL": "https://sho.rt/"})

			cfg, err := LoadServerConfig()

			if tt.wantErr {
				assert.EqualError(t, err, `invalid PORT "`+tt.port+`": must be a number between 1 and 65535`)
				assert.Equal(t, ServerConfig{}, cfg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.port, cfg.Port)
		})
	}
}

func TestLoadServerConfig_InvalidPortWithoutBaseURLReportsOnlyPortError(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "abc"})

	_, err := LoadServerConfig()

	assert.EqualError(t, err, `invalid PORT "abc": must be a number between 1 and 65535`)
}

func TestLoadServerConfig_ValidBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"https without trailing slash", "https://sho.rt", "https://sho.rt/"},
		{"https with trailing slash", "https://sho.rt/", "https://sho.rt/"},
		{"http with port", "http://localhost:8080", "http://localhost:8080/"},
		{"path prefix without slash", "https://example.com/s", "https://example.com/s/"},
		{"path prefix with slash", "https://example.com/s/", "https://example.com/s/"},
		{"nested path prefix", "https://example.com/a/b", "https://example.com/a/b/"},
		{"escaped path", "https://example.com/a%2Fb", "https://example.com/a%2Fb/"},
		{"uppercase scheme is lowercased", "HTTPS://sho.rt", "https://sho.rt/"},
		{"ipv4 host", "http://127.0.0.1:8080", "http://127.0.0.1:8080/"},
		{"ipv6 host", "http://[::1]:8080", "http://[::1]:8080/"},
		{"subdomain", "https://go.example.co.uk", "https://go.example.co.uk/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setServerEnv(t, map[string]string{"BASE_URL": tt.baseURL})

			cfg, err := LoadServerConfig()

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.BaseURL)
		})
	}
}

func TestLoadServerConfig_InvalidBaseURL(t *testing.T) {
	const notAbsolute = "must be an absolute http or https URL"
	tests := []struct {
		name    string
		baseURL string
		wantErr string
	}{
		{"missing scheme", "sho.rt", notAbsolute},
		{"host and port without scheme", "localhost:8080", notAbsolute},
		{"relative path", "/s/", notAbsolute},
		{"ftp scheme", "ftp://sho.rt", notAbsolute},
		{"mailto scheme", "mailto:admin@sho.rt", notAbsolute},
		{"javascript scheme", "javascript:alert(1)", notAbsolute},
		{"no host", "https://", notAbsolute},
		{"port without host", "http://:8080", notAbsolute},
		{"unparsable", "http://[::1", notAbsolute},
		{"query string", "https://sho.rt/?ref=1", "must not contain a query or fragment"},
		{"empty query", "https://sho.rt/?", "must not contain a query or fragment"},
		{"fragment", "https://sho.rt/#top", "must not contain a query or fragment"},
		{"username only", "https://admin@sho.rt", "must not contain credentials"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setServerEnv(t, map[string]string{"BASE_URL": tt.baseURL})

			cfg, err := LoadServerConfig()

			assert.EqualError(t, err, `invalid BASE_URL "`+tt.baseURL+`": `+tt.wantErr)
			assert.Equal(t, ServerConfig{}, cfg)
		})
	}
}

func TestLoadServerConfig_BaseURLCredentialsAreRedactedInError(t *testing.T) {
	setServerEnv(t, map[string]string{"BASE_URL": "https://admin:hunter2@sho.rt"})

	_, err := LoadServerConfig()

	require.Error(t, err)
	assert.EqualError(t, err, `invalid BASE_URL "https://admin:xxxxx@sho.rt": must not contain credentials`)
	assert.NotContains(t, err.Error(), "hunter2")
}

func TestLoadServerConfig_ReportsAllErrorsTogether(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "abc", "BASE_URL": "ftp://sho.rt", "APP_MODE": "prod"})

	cfg, err := LoadServerConfig()

	assert.EqualError(t, err, `invalid PORT "abc": must be a number between 1 and 65535`+"\n"+
		`invalid BASE_URL "ftp://sho.rt": must be an absolute http or https URL`+"\n"+
		`invalid APP_MODE "prod": must be one of debug, release, test`)
	assert.Equal(t, ServerConfig{}, cfg)
}

func TestServerConfig_Addr(t *testing.T) {
	assert.Equal(t, ":8080", ServerConfig{Port: "8080"}.Addr())
	assert.Equal(t, ":3000", ServerConfig{Port: "3000"}.Addr())
}

func TestLoad_Success(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "3000", "BASE_URL": "https://sho.rt", "APP_MODE": "release"})
	setEnv(t, validEnv())

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Config{
		Server: ServerConfig{Port: "3000", BaseURL: "https://sho.rt/", Mode: "release", RedirectStatus: 302},
		DB:     DBConfig{User: "user", Password: "secret", Host: "db", Port: "5432", Name: "main", SSLMode: "disable"},
	}, cfg)
}

func TestLoad_ServerErrorOnly(t *testing.T) {
	setServerEnv(t, map[string]string{"APP_MODE": "prod"})
	setEnv(t, validEnv())

	cfg, err := Load()

	assert.EqualError(t, err, `invalid APP_MODE "prod": must be one of debug, release, test`)
	assert.Equal(t, Config{}, cfg)
}

func TestLoad_DBErrorOnly(t *testing.T) {
	setServerEnv(t, map[string]string{})
	setEnv(t, withEnv(map[string]string{"DB_HOST": ""}))

	cfg, err := Load()

	assert.EqualError(t, err, "missing required environment variables: DB_HOST")
	assert.Equal(t, Config{}, cfg)
}

func TestLoad_ReportsServerAndDBErrorsTogether(t *testing.T) {
	setServerEnv(t, map[string]string{"PORT": "0"})
	setEnv(t, map[string]string{})

	cfg, err := Load()

	assert.EqualError(t, err, `invalid PORT "0": must be a number between 1 and 65535`+"\n"+
		"missing required environment variables: DB_USER, DB_PASSWORD, DB_HOST, DB_NAME")
	assert.Equal(t, Config{}, cfg)
}
