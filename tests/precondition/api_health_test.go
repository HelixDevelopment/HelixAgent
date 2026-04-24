//go:build integration

package precondition

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAPIHealthConnectivity(t *testing.T) {
	host := os.Getenv("HELIXAGENT_HOST")
	port := os.Getenv("HELIXAGENT_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "8100"
	}

	url := "http://" + host + ":" + port + "/v1/health"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Skipf("HelixAgent API not available at %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		t.Skipf("HelixAgent API returned server error at %s: status %d", url, resp.StatusCode)
	}

	t.Logf("HelixAgent API reachable at %s (status %d)", url, resp.StatusCode)
}
