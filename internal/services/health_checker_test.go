package services

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/sirupsen/logrus"
)

func TestNewServiceHealthChecker(t *testing.T) {
	logger := newTestLogger()
	hc := NewServiceHealthChecker(logger)

	if hc == nil {
		t.Fatal("Expected non-nil HealthChecker")
	}
	if hc.Logger != logger {
		t.Error("Expected Logger to be set")
	}
}

func TestCheckHTTP(t *testing.T) {
	logger := newTestLogger()
	hc := NewServiceHealthChecker(logger)

	t.Run("Healthy HTTP service", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))
		defer server.Close()

		// Parse the test server URL
		addr := strings.TrimPrefix(server.URL, "http://")

		ep := config.ServiceEndpoint{
			URL:        server.URL,
			HealthType: "http",
			HealthPath: "/",
			Timeout:    5 * time.Second,
		}

		err := hc.Check("test-http", ep)
		if err != nil {
			t.Errorf("Expected healthy HTTP check to pass, got: %v (addr=%s)", err, addr)
		}
	})

	t.Run("Unhealthy HTTP service (500)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ep := config.ServiceEndpoint{
			URL:        server.URL,
			HealthType: "http",
			HealthPath: "/health",
			Timeout:    5 * time.Second,
		}

		err := hc.Check("test-http-500", ep)
		if err == nil {
			t.Error("Expected error for 500 response")
		}
	})

	t.Run("Unreachable HTTP service", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping unreachable service test in short mode (may take 30+ seconds)")  // SKIP-OK: #short-mode
		}
		ep := config.ServiceEndpoint{
			Host:       "192.0.2.1",
			Port:       "59999",
			HealthType: "http",
			HealthPath: "/",
			Timeout:    1 * time.Second,
		}

		err := hc.Check("unreachable-http", ep)
		if err == nil {
			t.Error("Expected error for unreachable HTTP service")
		}
	})
}

func TestCheckTCP(t *testing.T) {
	logger := newTestLogger()
	hc := NewServiceHealthChecker(logger)

	t.Run("Healthy TCP service", func(t *testing.T) {
		// Start a TCP listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start test TCP listener: %v", err)
		}
		defer func() { _ = listener.Close() }()

		addr := listener.Addr().(*net.TCPAddr)

		ep := config.ServiceEndpoint{
			Host:       "127.0.0.1",
			Port:       strings.TrimPrefix(listener.Addr().String(), "127.0.0.1:"),
			HealthType: "tcp",
			Timeout:    5 * time.Second,
		}

		err = hc.Check("test-tcp", ep)
		if err != nil {
			t.Errorf("Expected healthy TCP check to pass (port %d), got: %v", addr.Port, err)
		}
	})

	t.Run("Unreachable TCP service", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping unreachable service test in short mode (may take 30+ seconds)")  // SKIP-OK: #short-mode
		}
		ep := config.ServiceEndpoint{
			Host:       "192.0.2.1",
			Port:       "59999",
			HealthType: "tcp",
			Timeout:    1 * time.Second,
		}

		err := hc.Check("unreachable-tcp", ep)
		if err == nil {
			t.Error("Expected error for unreachable TCP service")
		}
	})
}

func TestCheckWithRetry(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // suppress retry logs
	hc := NewServiceHealthChecker(logger)

	t.Run("Success on first try", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to start test listener: %v", err)
		}
		defer func() { _ = listener.Close() }()

		ep := config.ServiceEndpoint{
			Host:       "127.0.0.1",
			Port:       strings.TrimPrefix(listener.Addr().String(), "127.0.0.1:"),
			HealthType: "tcp",
			Timeout:    5 * time.Second,
			RetryCount: 3,
		}

		err = hc.CheckWithRetry("retry-success", ep)
		if err != nil {
			t.Errorf("Expected success, got: %v", err)
		}
	})

	t.Run("Failure after all retries", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping unreachable service test in short mode")  // SKIP-OK: #short-mode
		}
		ep := config.ServiceEndpoint{
			Host:       "192.0.2.1",
			Port:       "59999",
			HealthType: "tcp",
			Timeout:    500 * time.Millisecond,
			RetryCount: 2,
		}

		err := hc.CheckWithRetry("retry-fail", ep)
		if err == nil {
			t.Error("Expected error after all retries exhausted")
		}
		if !strings.Contains(err.Error(), "after 2 attempts") {
			t.Errorf("Expected error message to mention retry count, got: %v", err)
		}
	})

	t.Run("Zero retries defaults to 1", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping unreachable service test in short mode")  // SKIP-OK: #short-mode
		}
		ep := config.ServiceEndpoint{
			Host:       "192.0.2.1",
			Port:       "59999",
			HealthType: "tcp",
			Timeout:    500 * time.Millisecond,
			RetryCount: 0,
		}

		err := hc.CheckWithRetry("retry-zero", ep)
		if err == nil {
			t.Error("Expected error")
		}
		if !strings.Contains(err.Error(), "after 1 attempts") {
			t.Errorf("Expected 1 attempt for zero retry count, got: %v", err)
		}
	})
}

func TestTimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout handling test in short mode (requires unreachable network endpoints)")  // SKIP-OK: #short-mode
	}
	logger := newTestLogger()
	hc := NewServiceHealthChecker(logger)

	t.Run("Short timeout causes failure for slow service", func(t *testing.T) {
		ep := config.ServiceEndpoint{
			Host:       "192.0.2.1", // non-routable, will timeout
			Port:       "59999",
			HealthType: "tcp",
			Timeout:    100 * time.Millisecond,
		}

		start := time.Now()
		err := hc.Check("timeout-test", ep)
		elapsed := time.Since(start)

		if err == nil {
			t.Error("Expected timeout error")
		}
		// Should fail within a reasonable time of the timeout
		if elapsed > 5*time.Second {
			t.Errorf("Check took too long: %v", elapsed)
		}
	})
}

func TestCheckNoAddress(t *testing.T) {
	logger := newTestLogger()
	hc := NewServiceHealthChecker(logger)

	ep := config.ServiceEndpoint{
		HealthType: "tcp",
		Timeout:    1 * time.Second,
	}

	err := hc.Check("no-addr", ep)
	if err == nil {
		t.Error("Expected error for empty address")
	}
}
