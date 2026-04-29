//go:build integration

package precondition

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestRedisConnectivity(t *testing.T) {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "16379"
	}

	conn, err := net.DialTimeout("tcp", host+":"+port, 5*time.Second)
	if err != nil {
		t.Skipf("Redis not available at %s:%s: %v (SKIP-OK: #infra-redis-unavailable)", host, port, err)
	}
	conn.Close()
	t.Logf("Redis reachable at %s:%s", host, port)
}
