package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dbEnvKeys = []string{"DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB_NAME", "DB_SSLMODE"}

func validEnv() map[string]string {
	return map[string]string{
		"DB_USER":     "user",
		"DB_PASSWORD": "secret",
		"DB_HOST":     "db",
		"DB_PORT":     "5432",
		"DB_NAME":     "main",
		"DB_SSLMODE":  "disable",
	}
}

// setEnv sets every DB_* variable, using "" for keys not in env so values from
// the developer's shell never leak into a test.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, key := range dbEnvKeys {
		t.Setenv(key, env[key])
	}
}

func withEnv(overrides map[string]string) map[string]string {
	env := validEnv()
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func TestLoadDBConfig_AllValuesSet(t *testing.T) {
	setEnv(t, map[string]string{
		"DB_USER":     "app_user",
		"DB_PASSWORD": "p@ss$word",
		"DB_HOST":     "postgres.internal",
		"DB_PORT":     "6543",
		"DB_NAME":     "urls",
		"DB_SSLMODE":  "require",
	})

	cfg, err := LoadDBConfig()

	require.NoError(t, err)
	assert.Equal(t, DBConfig{
		User:     "app_user",
		Password: "p@ss$word",
		Host:     "postgres.internal",
		Port:     "6543",
		Name:     "urls",
		SSLMode:  "require",
	}, cfg)
}

func TestLoadDBConfig_Defaults(t *testing.T) {
	tests := []struct {
		name        string
		port        string
		sslMode     string
		wantPort    string
		wantSSLMode string
	}{
		{"both empty", "", "", "5432", "disable"},
		{"both whitespace", "  ", "\t", "5432", "disable"},
		{"port set, sslmode default", "6000", "", "6000", "disable"},
		{"sslmode set, port default", "", "verify-full", "5432", "verify-full"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, withEnv(map[string]string{"DB_PORT": tt.port, "DB_SSLMODE": tt.sslMode}))

			cfg, err := LoadDBConfig()

			require.NoError(t, err)
			assert.Equal(t, tt.wantPort, cfg.Port)
			assert.Equal(t, tt.wantSSLMode, cfg.SSLMode)
		})
	}
}

func TestLoadDBConfig_TrimsWhitespaceExceptCredentials(t *testing.T) {
	setEnv(t, map[string]string{
		"DB_USER":     " user ",
		"DB_PASSWORD": " secret ",
		"DB_HOST":     " db ",
		"DB_PORT":     " 5432 ",
		"DB_NAME":     " main ",
		"DB_SSLMODE":  " disable ",
	})

	cfg, err := LoadDBConfig()

	require.NoError(t, err)
	assert.Equal(t, "db", cfg.Host)
	assert.Equal(t, "5432", cfg.Port)
	assert.Equal(t, "main", cfg.Name)
	assert.Equal(t, "disable", cfg.SSLMode)
	// credentials may legitimately contain spaces, so they are kept as-is
	assert.Equal(t, " user ", cfg.User)
	assert.Equal(t, " secret ", cfg.Password)
}

func TestLoadDBConfig_MissingRequired(t *testing.T) {
	tests := []struct {
		name        string
		overrides   map[string]string
		wantMissing string
	}{
		{"user empty", map[string]string{"DB_USER": ""}, "DB_USER"},
		{"password empty", map[string]string{"DB_PASSWORD": ""}, "DB_PASSWORD"},
		{"host empty", map[string]string{"DB_HOST": ""}, "DB_HOST"},
		{"name empty", map[string]string{"DB_NAME": ""}, "DB_NAME"},
		{"user whitespace only", map[string]string{"DB_USER": "   "}, "DB_USER"},
		{"password whitespace only", map[string]string{"DB_PASSWORD": " \t "}, "DB_PASSWORD"},
		{"host whitespace only", map[string]string{"DB_HOST": " "}, "DB_HOST"},
		{"name whitespace only", map[string]string{"DB_NAME": "\n"}, "DB_NAME"},
		{"two missing", map[string]string{"DB_USER": "", "DB_NAME": ""}, "DB_USER, DB_NAME"},
		{
			"all missing",
			map[string]string{"DB_USER": "", "DB_PASSWORD": "", "DB_HOST": "", "DB_NAME": ""},
			"DB_USER, DB_PASSWORD, DB_HOST, DB_NAME",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, withEnv(tt.overrides))

			cfg, err := LoadDBConfig()

			require.Error(t, err)
			assert.EqualError(t, err, "missing required environment variables: "+tt.wantMissing)
			assert.Equal(t, DBConfig{}, cfg)
		})
	}
}

func TestLoadDBConfig_NothingSet(t *testing.T) {
	setEnv(t, map[string]string{})

	_, err := LoadDBConfig()

	assert.EqualError(t, err, "missing required environment variables: DB_USER, DB_PASSWORD, DB_HOST, DB_NAME")
}

func TestLoadDBConfig_Port(t *testing.T) {
	tests := []struct {
		port    string
		wantErr bool
	}{
		{"1", false},
		{"5432", false},
		{"65535", false},
		{"0", true},
		{"-1", true},
		{"65536", true},
		{"abc", true},
		{"5432x", true},
		{"54 32", true},
		{"5432.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			setEnv(t, withEnv(map[string]string{"DB_PORT": tt.port}))

			cfg, err := LoadDBConfig()

			if tt.wantErr {
				assert.EqualError(t, err, `invalid DB_PORT "`+tt.port+`": must be a number between 1 and 65535`)
				assert.Equal(t, DBConfig{}, cfg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.port, cfg.Port)
		})
	}
}

func TestLoadDBConfig_SSLMode(t *testing.T) {
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		t.Run("valid "+mode, func(t *testing.T) {
			setEnv(t, withEnv(map[string]string{"DB_SSLMODE": mode}))

			cfg, err := LoadDBConfig()

			require.NoError(t, err)
			assert.Equal(t, mode, cfg.SSLMode)
		})
	}

	for _, mode := range []string{"off", "DISABLE", "true", "verify"} {
		t.Run("invalid "+mode, func(t *testing.T) {
			setEnv(t, withEnv(map[string]string{"DB_SSLMODE": mode}))

			cfg, err := LoadDBConfig()

			assert.EqualError(t, err, `invalid DB_SSLMODE "`+mode+`"`)
			assert.Equal(t, DBConfig{}, cfg)
		})
	}
}

func TestLoadDBConfig_ErrorsDoNotLeakPassword(t *testing.T) {
	const password = "super-secret-value"
	cases := []map[string]string{
		{"DB_PASSWORD": password, "DB_USER": ""},
		{"DB_PASSWORD": password, "DB_PORT": "bad"},
		{"DB_PASSWORD": password, "DB_SSLMODE": "bad"},
	}

	for _, overrides := range cases {
		setEnv(t, withEnv(overrides))

		_, err := LoadDBConfig()

		require.Error(t, err)
		assert.NotContains(t, err.Error(), password)
	}
}

func TestDBConfig_DSN(t *testing.T) {
	cfg := DBConfig{User: "app_user", Password: "secret", Host: "db", Port: "5432", Name: "main", SSLMode: "disable"}

	assert.Equal(t, "postgres://app_user:secret@db:5432/main?sslmode=disable", cfg.DSN())
}

func TestDBConfig_DSN_EscapesSpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
	}{
		{"dollar sign", "app_user", "s3cr3t$456"},
		{"at sign", "app_user", "p@ssword"},
		{"colon and slash", "app_user", "a:b/c"},
		{"hash and question mark", "app_user", "a#b?c"},
		{"percent and space", "app_user", "100% sure"},
		{"special chars in user", "us@r:name", "secret"},
		{"unicode", "üser", "пароль"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DBConfig{User: tt.user, Password: tt.password, Host: "db", Port: "5432", Name: "main", SSLMode: "disable"}

			parsed, err := url.Parse(cfg.DSN())

			require.NoError(t, err)
			gotPassword, ok := parsed.User.Password()
			assert.True(t, ok)
			assert.Equal(t, tt.password, gotPassword)
			assert.Equal(t, tt.user, parsed.User.Username())
			assert.Equal(t, "db:5432", parsed.Host)
			assert.Equal(t, "/main", parsed.Path)
			assert.Equal(t, "disable", parsed.Query().Get("sslmode"))
		})
	}
}

func TestDBConfig_DSN_IPv6Host(t *testing.T) {
	cfg := DBConfig{User: "u", Password: "p", Host: "::1", Port: "5432", Name: "main", SSLMode: "disable"}

	parsed, err := url.Parse(cfg.DSN())

	require.NoError(t, err)
	assert.Equal(t, "[::1]:5432", parsed.Host)
	assert.Equal(t, "::1", parsed.Hostname())
	assert.Equal(t, "5432", parsed.Port())
}

func TestDBConfig_DSN_FromLoadedConfig(t *testing.T) {
	setEnv(t, map[string]string{
		"DB_USER":     "app_user",
		"DB_PASSWORD": "s3cr3t$456",
		"DB_HOST":     "db",
		"DB_NAME":     "main",
	})

	cfg, err := LoadDBConfig()
	require.NoError(t, err)

	assert.Equal(t, "postgres://app_user:s3cr3t$456@db:5432/main?sslmode=disable", cfg.DSN())
}
