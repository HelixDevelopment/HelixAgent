package placement

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// fakeExecutor satisfies remote.RemoteExecutor with synthetic
// /proc/stat etc. output so the host prober succeeds without SSH.
// Every host receives the same canned snapshot — the placement tests
// here care about WHERE services land, not the snapshot's specifics.
type fakeExecutor struct{}

func (fakeExecutor) Execute(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
	canned := strings.Join([]string{
		// /proc/stat
		"cpu  100 0 50 850 0 0 0 0 0 0",
		"cpu0 25 0 12 213 0 0 0 0 0 0",
		"---SEPARATOR---",
		// /proc/meminfo
		"MemTotal:       16384000 kB",
		"MemFree:        12000000 kB",
		"MemAvailable:   13000000 kB",
		"Buffers:               0 kB",
		"Cached:           500000 kB",
		"SwapTotal:             0 kB",
		"SwapFree:              0 kB",
		"---SEPARATOR---",
		// /proc/loadavg
		"0.50 0.40 0.30 1/200 12345",
		"---SEPARATOR---",
		// df
		"100000M     20000M",
		"---SEPARATOR---",
		// nproc
		"4",
		"---SEPARATOR---",
		// /proc/net/dev
		"  eth0:  100000      10    0    0    0     0          0         0   200000      20    0    0    0     0       0          0",
	}, "\n")
	return &remote.CommandResult{Stdout: canned, ExitCode: 0}, nil
}
func (fakeExecutor) ExecuteStream(context.Context, remote.RemoteHost, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (fakeExecutor) CopyFile(context.Context, remote.RemoteHost, string, string) error {
	return nil
}
func (fakeExecutor) CopyDir(context.Context, remote.RemoteHost, string, string) error {
	return nil
}
func (fakeExecutor) IsReachable(context.Context, remote.RemoteHost) bool {
	return true
}
func (fakeExecutor) Close() error { return nil }

// TestPlanCompose_CoLocationStaysTogether asserts that services in the
// same depends_on group land on the SAME host. Without this property,
// cognee on host A would try to talk to postgres on host B and writes
// would diverge across hosts (the data-consistency invariant).
func TestPlanCompose_CoLocationStaysTogether(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(src, []byte(`
services:
  postgres:
    image: postgres:15
    deploy: { resources: { limits: { memory: 4G, cpus: "2.0" }, reservations: { memory: 1G, cpus: "0.5" } } }
  redis:
    image: redis:7
    deploy: { resources: { limits: { memory: 1G, cpus: "0.5" }, reservations: { memory: 256M, cpus: "0.1" } } }
  cognee:
    image: cognee:latest
    depends_on: [postgres, redis]
    deploy: { resources: { limits: { memory: 4G, cpus: "2.0" }, reservations: { memory: 1G, cpus: "0.5" } } }
  mock-llm:
    image: mock:latest
    deploy: { resources: { limits: { memory: 1G, cpus: "0.5" }, reservations: { memory: 256M, cpus: "0.1" } } }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	hm := remote.NewHostManager(fakeExecutor{}, nil)
	if err := hm.AddHost(remote.RemoteHost{Name: "thinker", Address: "thinker.local", User: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := hm.AddHost(remote.RemoteHost{Name: "amber", Address: "amber.local", User: "test"}); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCompose(context.Background(), src, "", hm,
		scheduler.WithStrategy(scheduler.StrategyRoundRobin))
	if err != nil {
		t.Fatalf("PlanCompose: %v", err)
	}

	// Every service from the cognee co-location group ({cognee,
	// postgres, redis}) must land on the same host.
	hostOf := make(map[string]string)
	for _, d := range plan.Decisions {
		hostOf[d.Requirement.Name] = d.HostName
	}
	if hostOf["postgres"] != hostOf["cognee"] {
		t.Errorf("postgres host=%q != cognee host=%q",
			hostOf["postgres"], hostOf["cognee"])
	}
	if hostOf["redis"] != hostOf["cognee"] {
		t.Errorf("redis host=%q != cognee host=%q",
			hostOf["redis"], hostOf["cognee"])
	}
}

// TestPlanCompose_NoDuplicates asserts every service appears in
// exactly one HostAssignment. This is the core invariant the user
// asked for: zero duplicate containers across hosts.
func TestPlanCompose_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(src, []byte(`
services:
  a: { image: a:1 }
  b: { image: b:1 }
  c: { image: c:1 }
  d: { image: d:1 }
  e: { image: e:1 }
  f: { image: f:1 }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	hm := remote.NewHostManager(fakeExecutor{}, nil)
	_ = hm.AddHost(remote.RemoteHost{Name: "h1", Address: "h1.local", User: "test"})
	_ = hm.AddHost(remote.RemoteHost{Name: "h2", Address: "h2.local", User: "test"})

	plan, err := PlanCompose(context.Background(), src, "", hm,
		scheduler.WithStrategy(scheduler.StrategyRoundRobin))
	if err != nil {
		t.Fatalf("PlanCompose: %v", err)
	}

	seen := make(map[string]string)
	for _, a := range plan.Assignments {
		for _, s := range a.ServiceList {
			if prev, ok := seen[s]; ok {
				t.Errorf("service %q appears on %q AND %q", s, prev, a.HostName)
			}
			seen[s] = a.HostName
		}
	}
	if len(seen) != 6 {
		t.Errorf("placed %d services, want 6", len(seen))
	}
}

// TestPlanCompose_AllServicesPlaced asserts no service is left without
// a host. Mirrors a regression where empty-host scheduling silently
// dropped containers.
func TestPlanCompose_AllServicesPlaced(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(src, []byte(`
services:
  one:   { image: a:1 }
  two:   { image: b:1 }
  three: { image: c:1 }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	hm := remote.NewHostManager(fakeExecutor{}, nil)
	_ = hm.AddHost(remote.RemoteHost{Name: "h1", Address: "h1.local", User: "test"})

	plan, err := PlanCompose(context.Background(), src, "", hm,
		scheduler.WithStrategy(scheduler.StrategyResourceAware))
	if err != nil {
		t.Fatalf("PlanCompose: %v", err)
	}

	placed := 0
	for _, a := range plan.Assignments {
		placed += len(a.ServiceList)
	}
	if placed != 3 {
		t.Errorf("placed %d services, want 3 (no service should be lost)", placed)
	}
}
