package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := New("test")
	assert.Equal(t, "test", cfg.ServiceName)
	assert.Equal(t, "localhost:50051", cfg.Address)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 10*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.RetryBackoff)
	assert.Equal(t, 30*time.Second, cfg.MaxRetryBackoff)
	assert.False(t, cfg.EnableTLS)
	assert.Equal(t, 64*1024*1024, cfg.MaxRecvMsgSize)
	assert.Equal(t, 16*1024*1024, cfg.MaxSendMsgSize)
	assert.Empty(t, cfg.AuthToken)
}

func TestNewWithOptions(t *testing.T) {
	cfg := New("autotemp",
		WithAddress("autotemp.example.com:443"),
		WithTimeout(45*time.Second),
		WithConnectionTimeout(15*time.Second),
		WithMaxRetries(5),
		WithRetryBackoff(2*time.Second),
		WithMaxRetryBackoff(60*time.Second),
		WithTLS("/certs/cert.pem", "/certs/key.pem", "/certs/ca.pem"),
		WithAuthToken("secret-token"),
		WithCompression("gzip"),
		WithKeepalive(5*time.Second, 3*time.Second),
		WithMaxMessageSize(128*1024*1024),
		WithMetadata("x-request-id", "test-123"),
	)

	assert.Equal(t, "autotemp.example.com:443", cfg.Address)
	assert.Equal(t, 45*time.Second, cfg.Timeout)
	assert.Equal(t, 15*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.RetryBackoff)
	assert.Equal(t, 60*time.Second, cfg.MaxRetryBackoff)
	assert.True(t, cfg.EnableTLS)
	assert.Equal(t, "/certs/cert.pem", cfg.TLSCertPath)
	assert.Equal(t, "secret-token", cfg.AuthToken)
	assert.Equal(t, "gzip", cfg.Compression)
	assert.Equal(t, 5*time.Second, cfg.KeepaliveTime)
	assert.Equal(t, 3*time.Second, cfg.KeepaliveTimeout)
	assert.Equal(t, 128*1024*1024, cfg.MaxRecvMsgSize)
	assert.Equal(t, 128*1024*1024, cfg.MaxSendMsgSize)
	assert.Equal(t, "test-123", cfg.Metadata["x-request-id"])
}

func TestFromEnv(t *testing.T) {
	// Set test environment variables
	os.Setenv("TESTSRV_ADDRESS", "test.example.com:8080")
	os.Setenv("TESTSRV_TIMEOUT", "45s")
	os.Setenv("TESTSRV_MAX_RETRIES", "7")
	os.Setenv("TESTSRV_ENABLE_TLS", "true")
	os.Setenv("TESTSRV_AUTH_TOKEN", "env-token")
	os.Setenv("TESTSRV_COMPRESSION", "gzip")
	defer func() {
		os.Unsetenv("TESTSRV_ADDRESS")
		os.Unsetenv("TESTSRV_TIMEOUT")
		os.Unsetenv("TESTSRV_MAX_RETRIES")
		os.Unsetenv("TESTSRV_ENABLE_TLS")
		os.Unsetenv("TESTSRV_AUTH_TOKEN")
		os.Unsetenv("TESTSRV_COMPRESSION")
	}()

	cfg := FromEnv("testsrv")
	assert.Equal(t, "test.example.com:8080", cfg.Address)
	assert.Equal(t, 45*time.Second, cfg.Timeout)
	assert.Equal(t, 7, cfg.MaxRetries)
	assert.True(t, cfg.EnableTLS)
	assert.Equal(t, "env-token", cfg.AuthToken)
	assert.Equal(t, "gzip", cfg.Compression)
}

func TestFromFile(t *testing.T) {
	yaml := `
service_name: test
address: file.example.com:9090
timeout: 20s
connection_timeout: 5s
max_retries: 2
retry_backoff: 500ms
enable_tls: false
auth_token: file-token
metadata:
  x-env: production
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0644))

	cfg, err := FromFile(configPath, "test")
	require.NoError(t, err)
	assert.Equal(t, "test", cfg.ServiceName)
	assert.Equal(t, "file.example.com:9090", cfg.Address)
	assert.Equal(t, 20*time.Second, cfg.Timeout)
	assert.Equal(t, 5*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 2, cfg.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, cfg.RetryBackoff)
	assert.False(t, cfg.EnableTLS)
	assert.Equal(t, "file-token", cfg.AuthToken)
	assert.Equal(t, "production", cfg.Metadata["x-env"])
}

func TestFromFileServiceSpecific(t *testing.T) {
	yaml := `
autotemp:
  address: autotemp.example.com:50051
  timeout: 30s
  max_retries: 3

obliteratus:
  address: obliteratus.example.com:50052
  timeout: 60s
  max_retries: 5
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0644))

	cfg, err := FromFile(configPath, "autotemp")
	require.NoError(t, err)
	assert.Equal(t, "autotemp", cfg.ServiceName)
	assert.Equal(t, "autotemp.example.com:50051", cfg.Address)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 3, cfg.MaxRetries)

	cfg2, err := FromFile(configPath, "obliteratus")
	require.NoError(t, err)
	assert.Equal(t, "obliteratus", cfg2.ServiceName)
	assert.Equal(t, "obliteratus.example.com:50052", cfg2.Address)
	assert.Equal(t, 60*time.Second, cfg2.Timeout)
	assert.Equal(t, 5, cfg2.MaxRetries)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "valid config",
			cfg:     New("test"),
			wantErr: "",
		},
		{
			name:    "missing service name",
			cfg:     New(""),
			wantErr: "service name is required",
		},
		{
			name:    "missing address",
			cfg:     &Config{ServiceName: "test", Address: "", Timeout: 1, ConnectionTimeout: 1, RetryBackoff: 1},
			wantErr: "address is required",
		},
		{
			name:    "zero timeout",
			cfg:     &Config{ServiceName: "test", Address: ":50051", Timeout: 0, ConnectionTimeout: 1, RetryBackoff: 1},
			wantErr: "timeout must be positive",
		},
		{
			name: "TLS without cert",
			cfg: func() *Config {
				c := New("test")
				c.EnableTLS = true
				return c
			}(),
			wantErr: "tls_cert_path is required when TLS is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestEnvPrefix(t *testing.T) {
	cfg := New("autotemp")
	assert.Equal(t, "AUTOTEMP_", cfg.EnvPrefix())
}
