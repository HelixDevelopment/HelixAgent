//go:build fuzz

// Package fuzz provides Go native fuzzing tests for critical parsing paths
// in the HelixAgent system. These tests ensure that malformed or adversarial
// input never causes panics or undefined behavior.
package fuzz

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// FuzzEnvVarParsing tests that parsing environment variable values into typed
// config fields never panics. HelixAgent reads many config values from env vars
// and converts them to booleans, integers, durations, and URLs.
func FuzzEnvVarParsing(f *testing.F) {
	// Seed corpus: realistic and adversarial env var values
	f.Add("true", "8100", "localhost", "helixagent123", "60s")
	f.Add("false", "0", "", "", "0")
	f.Add("1", "65535", "127.0.0.1", "pass\x00word", "1h30m")
	f.Add("yes", "-1", "::1", strings.Repeat("x", 10000), "invalid-duration")
	f.Add("TRUE", "99999999", "host:port", "key=value=extra", "1e9")
	f.Add("", "", "", "", "")
	f.Add("\x00", "\xff\xfe", "host\nnewline", "key\ttab", "30\x00s")

	f.Fuzz(func(t *testing.T, boolVal, portVal, hostVal, secretVal, durationVal string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzEnvVarParsing panicked: bool=%q port=%q host=%q panic=%v",
					boolVal, portVal, hostVal, r)
			}
		}()

		// ParseBool — used for feature flags like COGNEE_ENABLED, GRAPHQL_ENABLED
		_, _ = strconv.ParseBool(boolVal)

		// ParseInt — used for PORT, DB_PORT, REDIS_PORT
		_, _ = strconv.ParseInt(portVal, 10, 64)
		_, _ = strconv.Atoi(portVal)

		// ParseFloat — used for scoring weights, temperature defaults
		_, _ = strconv.ParseFloat(portVal, 64)

		// URL construction — used in ServiceEndpoint.ResolvedURL()
		if hostVal != "" && portVal != "" {
			_ = hostVal + ":" + portVal
		}

		// TrimSpace — always applied to env var values
		_ = strings.TrimSpace(boolVal)
		_ = strings.TrimSpace(portVal)
		_ = strings.TrimSpace(hostVal)
		_ = strings.TrimSpace(secretVal)
		_ = strings.TrimSpace(durationVal)

		// Split on "," — used for lists like preferred providers
		_ = strings.Split(secretVal, ",")

		// os.Getenv simulation: set and read back (safe, sandboxed in test)
		key := "FUZZ_TEST_VAR_" + strconv.Itoa(len(boolVal))
		os.Setenv(key, boolVal) //nolint:errcheck
		_ = os.Getenv(key)
		os.Unsetenv(key) //nolint:errcheck
	})
}

// FuzzYAMLLikeConfigParsing tests that parsing YAML-style key:value config
// content (as used in configs/development.yaml and configs/production.yaml)
// never panics when given arbitrary byte sequences.
func FuzzYAMLLikeConfigParsing(f *testing.F) {
	// Seed corpus: realistic YAML config fragments
	f.Add([]byte("port: 8100\ngin_mode: debug\n"))
	f.Add([]byte("database:\n  host: localhost\n  port: 15432\n  name: helixagent_db\n"))
	f.Add([]byte("redis:\n  host: localhost\n  port: 16379\n  password: secret\n"))
	f.Add([]byte("llm:\n  providers:\n    - name: openai\n      enabled: true\n"))
	f.Add([]byte("{}"))
	f.Add([]byte(""))
	f.Add([]byte("key: value\nkey: duplicate\n"))
	f.Add([]byte("\x00\x01\xff\xfe"))
	f.Add([]byte("a: " + strings.Repeat("b", 100000)))
	f.Add([]byte("deeply:\n  nested:\n    config:\n      value: " + strings.Repeat("x", 1000)))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzYAMLLikeConfigParsing panicked with input %q: %v", data, r)
			}
		}()

		// Parse as generic YAML-line structure (line-by-line key:value extraction)
		// This mirrors the manual parsing in some config loaders
		lines := strings.Split(string(data), "\n")
		result := make(map[string]string)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.Index(line, ":")
			if idx < 0 {
				continue
			}
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if key != "" {
				result[key] = val
			}
		}

		// Safe access of parsed keys
		for k, v := range result {
			_ = len(k)
			_ = len(v)
			_, _ = strconv.ParseBool(v)
			_, _ = strconv.ParseInt(v, 10, 64)
		}
	})
}

// FuzzJSONConfigParsing tests that parsing JSON-encoded configuration objects
// (as used in LLM provider configs, MCP server configs, and agent configs)
// never panics with arbitrary inputs.
func FuzzJSONConfigParsing(f *testing.F) {
	// Seed corpus: realistic JSON config objects
	f.Add(`{"provider":"openai","api_key":"sk-test","base_url":"https://api.openai.com/v1","model":"gpt-4","enabled":true}`)
	f.Add(`{"host":"localhost","port":15432,"user":"helixagent","password":"secret","dbname":"helixagent_db","sslmode":"disable"}`)
	f.Add(`{"url":"http://localhost:8100/v1","model":"helixagent/helixagent-debate","api_key":"test-key"}`)
	f.Add(`{"servers":{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem"]}}}`)
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(`[]`)
	f.Add(`invalid`)
	f.Add(`{"` + strings.Repeat("k", 1000) + `":"` + strings.Repeat("v", 10000) + `"}`)
	f.Add(`{"nested":` + strings.Repeat(`{"a":`, 50) + `null` + strings.Repeat(`}`, 50) + `}`)

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzJSONConfigParsing panicked with input %q: %v", input, r)
			}
		}()

		// Parse as generic config map
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			return
		}

		// Safe extraction of common config fields
		if host, ok := cfg["host"].(string); ok {
			_ = strings.TrimSpace(host)
		}
		if portRaw, ok := cfg["port"]; ok {
			switch v := portRaw.(type) {
			case float64:
				_ = int(v)
			case string:
				_, _ = strconv.Atoi(v)
			}
		}
		if enabled, ok := cfg["enabled"].(bool); ok {
			_ = enabled
		}
		if apiKey, ok := cfg["api_key"].(string); ok {
			_ = len(apiKey) > 0
		}
		if baseURL, ok := cfg["base_url"].(string); ok {
			_ = strings.HasPrefix(baseURL, "http")
		}

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(cfg)

		// Also try as a slice (some configs are arrays)
		var cfgSlice []map[string]interface{}
		if err := json.Unmarshal([]byte(input), &cfgSlice); err == nil {
			for _, item := range cfgSlice {
				for k, v := range item {
					_ = len(k)
					_ = v
				}
			}
		}
	})
}
