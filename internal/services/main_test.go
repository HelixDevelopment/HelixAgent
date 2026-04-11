package services

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// NOTE: goleak.VerifyTestMain calls m.Run() internally.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("time.Sleep"),
		goleak.IgnoreAnyFunction("net.(*netFD).Read"),
		goleak.IgnoreAnyFunction("net.(*netFD).Accept"),
		// Background goroutines from services tested in this package
		goleak.IgnoreTopFunction("dev.helix.agent/internal/services.(*ProtocolMonitor).alertChecker"),
		goleak.IgnoreTopFunction("dev.helix.agent/internal/services.(*ProtocolMonitor).metricsCollector"),
		goleak.IgnoreTopFunction("dev.helix.agent/internal/verifier.(*ModelDiscoveryService).discoveryLoop"),
		goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/maintnotifications.(*CircuitBreakerManager).cleanupLoop"),
		goleak.IgnoreTopFunction("dev.helix.agent/internal/services.(*DebateMonitoringService).runMonitoringLoop"),
		goleak.IgnoreTopFunction("net/http.(*http2ClientConn).readLoop"),
		goleak.IgnoreTopFunction("crypto/tls.(*Conn).Read"),
		// Go's net.Resolver.lookupIPAddr spawns a background goroutine
		// (lookup.go:339 / singleflight.doCall) to race DNS lookups.
		// When a test finishes before the DNS RTT completes, the
		// goroutine is still parked in `chan receive`. It is a runtime
		// detail of Go's resolver — not a genuine leak — so ignore it.
		goleak.IgnoreTopFunction("net.(*Resolver).lookupIPAddr.func2"),
		goleak.IgnoreTopFunction("internal/singleflight.(*Group).doCall"),
	)
}
