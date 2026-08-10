package ports

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Drift guards for every artifact OUTSIDE Go that publishes the gRPC port.
//
// THE DEFECT CLASS. The gRPC port has now been wrong in three different
// spellings across the tree, each wrong in its own way, and none of them
// detectable by any Go test — because no Go test ever read the artifacts
// that carried them:
//
//	:50051                  cmd/grpc-server (fixed, ec95a277) — the gRPC
//	                        ecosystem's conventional port, held on this host
//	                        by helixcode-infra-weaviate
//	GRPC_PORT=9090          k8s/base/deployment.yaml — an env var NO Go source
//	                        reads, paired with containerPort 9090, so the
//	                        manifest published a gRPC endpoint that nothing
//	                        could ever bind
//	HELIXAGENT_GRPC_PORT    challenges/scripts/protocol_grpc_challenge.sh —
//	  :-7062                a THIRD spelling, defaulting to nobody's port
//
// internal/ports exists precisely so port ownership is arbitrated in one
// place, and TestOffsets_NoCollisions enforces that for services inside the
// registry. But a YAML manifest or a bash script naming a raw integer is
// invisible to it: the registry cannot arbitrate a number it never sees.
// These tests close that gap by reading the published artifacts and holding
// them to ports.Default(HelixAgentGRPC) — so the registry becomes the single
// source of truth in fact, not merely by intention.
//
// §11.4.115 polarity — RED_MODE=1 asserts the defect IS present (reproduce on
// a pre-fix tree); RED_MODE=0 (the default) asserts it is absent.

// repoRoot returns the repository root relative to this package's directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}

// readRepoFile reads a repo-relative file, failing loudly when it is missing.
//
// A missing file must never be silently tolerated: these guards exist to
// catch drift in published artifacts, and "the artifact moved" would
// otherwise turn every assertion below into a vacuous pass (§11.4.201 — a
// guard that cannot see its subject must not report a verdict).
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	require.NoErrorf(t, err, "cannot read %s; if it moved, update this guard "+
		"rather than deleting it — the drift it catches is still real", rel)
	return string(b)
}

func redModeOn() bool { return strings.TrimSpace(os.Getenv("RED_MODE")) == "1" }

// deadGRPCEnvVar is the variable the k8s Deployment used to set. No Go source
// reads it, so it could never influence the port the server binds.
const deadGRPCEnvVar = "GRPC_PORT"

// legacyManifestGRPCPort is the number the k8s manifests published for gRPC.
// It is not the registry port and nothing ever bound it.
const legacyManifestGRPCPort = "9090"

// k8sManifests are every manifest that names the gRPC port. They must agree
// with each other AND with the registry: a Service whose targetPort points at
// a containerPort no process binds is a blackhole endpoint, and the Deployment
// and Service disagreeing is the same defect with an extra hop.
var k8sManifests = []string{
	"k8s/base/deployment.yaml",
	"k8s/base/service.yaml",
	"k8s/base/networkpolicy.yaml",
	"k8s/base/configmap.yaml",
}

// TestK8sManifests_GRPCPortMatchesRegistry asserts the k8s manifests publish
// the registry's gRPC port and do not carry the dead env var.
//
// WHY THE ENV VAR MATTERS AS MUCH AS THE NUMBER. Setting GRPC_PORT=9090 looks
// like configuration but is inert: the server resolves ports.HelixAgentGRPC,
// whose variable is HELIXAGENT_PORT_GRPC. An operator reading the manifest
// would reasonably believe the port was pinned, and changing it would have no
// effect whatsoever — configuration that silently does nothing is worse than
// no configuration, because it defeats the next person's debugging.
func TestK8sManifests_GRPCPortMatchesRegistry(t *testing.T) {
	want := strconv.Itoa(Default(HelixAgentGRPC))

	if redModeOn() {
		dep := readRepoFile(t, "k8s/base/deployment.yaml")
		require.Contains(t, dep, deadGRPCEnvVar,
			"RED_MODE=1: the pre-fix Deployment pins the dead %s variable", deadGRPCEnvVar)
		require.Contains(t, dep, "containerPort: "+legacyManifestGRPCPort,
			"RED_MODE=1: the pre-fix Deployment exposes containerPort %s, which no "+
				"process binds — the server resolves the registry port %s",
			legacyManifestGRPCPort, want)
		require.NotEqual(t, legacyManifestGRPCPort, want,
			"the legacy manifest port and the registry port must differ, or this "+
				"guard proves nothing")
		return
	}

	for _, rel := range k8sManifests {
		body := readRepoFile(t, rel)

		// The dead variable must be gone. Assert on non-comment lines so the
		// manifests may still DOCUMENT the old name without failing the guard.
		for i, line := range strings.Split(body, "\n") {
			code := line
			if idx := strings.Index(code, "#"); idx >= 0 {
				code = code[:idx]
			}
			require.NotContainsf(t, code, deadGRPCEnvVar,
				"%s:%d sets %s, which no Go source reads; the server honours %s",
				rel, i+1, deadGRPCEnvVar, string(HelixAgentGRPC))
		}

		// Every gRPC-adjacent port must be the registry port. Checking the
		// legacy number specifically keeps the failure message actionable.
		require.NotContainsf(t, stripComments(body), legacyManifestGRPCPort,
			"%s still publishes the legacy gRPC port %s; the registry port is %s",
			rel, legacyManifestGRPCPort, want)
	}

	// The Deployment must expose the registry port, and the Service must route
	// to that same containerPort — otherwise the Service is a blackhole.
	dep := readRepoFile(t, "k8s/base/deployment.yaml")
	require.Containsf(t, dep, "containerPort: "+want,
		"deployment.yaml must expose the registry gRPC port %s", want)

	svc := readRepoFile(t, "k8s/base/service.yaml")
	require.Containsf(t, svc, "targetPort: "+want,
		"service.yaml must route to the registry gRPC port %s, or it forwards "+
			"traffic to a port no container binds", want)
}

// stripComments removes YAML/shell comments so assertions read code, not prose.
func stripComments(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// challengeGRPCPortPattern captures the challenge script's port default.
var challengeGRPCPortPattern = regexp.MustCompile(
	`GRPC_PORT="\$\{HELIXAGENT_PORT_GRPC:-(\d+)\}"`)

// TestChallengeScript_GRPCPortMatchesRegistry asserts the gRPC challenge's
// port default equals the registry default.
//
// A bash challenge cannot call into Go, so the number is necessarily
// duplicated in the script. This test is what makes that duplication safe:
// it is the mechanical link that stops the challenge drifting from the server
// again, which is exactly how it came to probe :7062 — a port belonging to no
// service at all — and certify gRPC healthy against it.
func TestChallengeScript_GRPCPortMatchesRegistry(t *testing.T) {
	const rel = "challenges/scripts/protocol_grpc_challenge.sh"
	body := readRepoFile(t, rel)

	m := challengeGRPCPortPattern.FindStringSubmatch(body)
	require.Lenf(t, m, 2,
		"%s must resolve its port as GRPC_PORT=\"${%s:-<registry-default>}\"; "+
			"a different spelling is how it drifted from the server before",
		rel, string(HelixAgentGRPC))

	require.Equalf(t, strconv.Itoa(Default(HelixAgentGRPC)), m[1],
		"%s defaults to gRPC port %s but the registry default is %d", rel, m[1],
		Default(HelixAgentGRPC))
}
