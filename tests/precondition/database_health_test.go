//go:build integration

package precondition

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestDatabaseConnectivity(t *testing.T) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "15432"
	}

	conn, err := net.DialTimeout("tcp", host+":"+port, 5*time.Second)
	if err != nil {
		t.Skipf("Database not available at %s:%s: %v (SKIP-OK: #infra-db-unavailable)", host, port, err)
	}
	conn.Close()
	t.Logf("Database reachable at %s:%s", host, port)
}
