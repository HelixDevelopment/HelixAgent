package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	containeradapter "dev.helix.agent/internal/adapters/containers"
	"dev.helix.agent/internal/placement"
	"digital.vasic.containers/pkg/scheduler"
	"github.com/sirupsen/logrus"
)

// deployComposePartitioned distributes a compose file across remote
// hosts so each service runs on EXACTLY ONE host (no replication).
// Replaces the broadcast-style RemoteComposeUp for partitioned
// distribution.
//
// Flow:
//  1. Parse the compose file → ContainerRequirements (with co-location
//     groups derived from depends_on).
//  2. Use placement.PlanCompose → scheduler picks a host per group.
//  3. For each HostAssignment, emit a per-host filtered compose file
//     containing ONLY the services placed on that host.
//  4. Ship each per-host compose to its target host via the adapter's
//     existing build-context-aware DeployComposeToHost.
//
// The placement plan (host -> services) is written to
// `<project>/.placement-plan.json` for the gateway and the Challenge
// to read. Service-host pairs are also exported as
// `SVC_<SERVICE>_HOST=<host>` env vars so internal/config picks them
// up at runtime.
//
// Returns the realized HostAssignments so callers can populate the
// service registry / monitor health.
func deployComposePartitioned(
	ctx context.Context,
	adapter *containeradapter.Adapter,
	composeFile, profile string,
	logger *logrus.Logger,
) ([]placement.HostAssignment, error) {

	if adapter == nil || !adapter.RemoteEnabled() {
		return nil, fmt.Errorf("partitioned deploy requires remote distribution")
	}
	hm := adapter.HostManager()
	if hm == nil {
		return nil, fmt.Errorf("adapter has no host manager")
	}

	absCompose, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("resolve compose path: %w", err)
	}

	plan, err := placement.PlanCompose(ctx, absCompose, profile, hm,
		scheduler.WithStrategy(scheduler.StrategyResourceAware))
	if err != nil {
		return nil, fmt.Errorf("plan compose: %w", err)
	}
	if len(plan.Assignments) == 0 {
		logger.WithField("compose", composeFile).
			Warn("placement plan is empty; nothing to deploy")
		return nil, nil
	}

	// Log the plan up-front so operators can audit placement before
	// the (slow) deploy phase runs.
	for _, a := range plan.Assignments {
		logger.WithFields(logrus.Fields{
			"host":      a.HostName,
			"compose":   filepath.Base(absCompose),
			"profile":   profile,
			"services":  len(a.ServiceList),
		}).Info("placement: assigning services to host")
	}

	// Persist the plan for downstream consumers (gateway service
	// registry, partitioned_distribution_challenge.sh, operator
	// triage).
	if err := writePlanFile(absCompose, plan); err != nil {
		logger.WithError(err).Warn("failed to persist placement plan; continuing")
	}

	// Stage per-host filtered compose files in a temp dir under the
	// project root so the adapter's build-context resolver finds them.
	stageDir, err := os.MkdirTemp(filepath.Dir(absCompose), ".placement-")
	if err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stageDir)
	}()

	// Stable host iteration so logs and deploys are deterministic.
	sort.Slice(plan.Assignments, func(i, j int) bool {
		return plan.Assignments[i].HostName < plan.Assignments[j].HostName
	})

	var deployErrs []string
	out := make([]placement.HostAssignment, 0, len(plan.Assignments))

	for _, a := range plan.Assignments {
		// Emit per-host filtered compose at the SAME relative position
		// in the project tree so build contexts (e.g. `context: ../..`
		// in MCP compose) resolve correctly on remote.
		composeRel, _ := filepath.Rel(
			filepath.Dir(absCompose),
			absCompose,
		)
		_ = composeRel
		hostFile := filepath.Join(
			stageDir, a.HostName,
			filepath.Base(absCompose),
		)
		if _, err := placement.EmitPerHostCompose(
			absCompose, a.ServiceList, hostFile,
		); err != nil {
			deployErrs = append(deployErrs,
				fmt.Sprintf("emit %s: %v", a.HostName, err))
			continue
		}
		a.OutFile = hostFile

		// Replace the source compose with the per-host file at the
		// SAME relative path inside the project so the build-context
		// adapter copies it to <remote>/<rel-path-from-project> and
		// `context: ../..` etc. still resolve.
		stagedAtCanonical := filepath.Join(
			filepath.Dir(absCompose),
			fmt.Sprintf(".placement-%s-%s",
				a.HostName, filepath.Base(absCompose)),
		)
		if err := copyFileSimple(hostFile, stagedAtCanonical); err != nil {
			deployErrs = append(deployErrs,
				fmt.Sprintf("stage %s: %v", a.HostName, err))
			continue
		}

		logger.WithFields(logrus.Fields{
			"host":     a.HostName,
			"file":     stagedAtCanonical,
			"services": len(a.ServiceList),
		}).Info("placement: deploying per-host compose")

		if err := adapter.DeployComposeToHost(
			ctx, a.HostName, stagedAtCanonical, profile,
		); err != nil {
			deployErrs = append(deployErrs,
				fmt.Sprintf("deploy %s: %v", a.HostName, err))
			_ = os.Remove(stagedAtCanonical)
			continue
		}
		_ = os.Remove(stagedAtCanonical)

		// Export service-host bindings for the gateway. This wires
		// SVC_<SERVICE>_HOST so internal/config/services.go connects
		// to the right host at runtime.
		for _, svc := range a.ServiceList {
			envKey := serviceHostEnvKey(svc)
			_ = os.Setenv(envKey, hostAddress(adapter, a.HostName))
		}

		out = append(out, a)
	}

	if len(deployErrs) > 0 {
		return out, fmt.Errorf("partitioned deploy errors: %v", deployErrs)
	}
	return out, nil
}

// writePlanFile records the placement plan as JSON next to the project
// for operators and the Challenge to inspect.
func writePlanFile(composePath string, plan *placement.Plan) error {
	planFile := filepath.Join(
		filepath.Dir(composePath),
		fmt.Sprintf(".placement-plan-%s.json",
			sanitizeName(filepath.Base(composePath))),
	)
	return placement.WritePlanJSON(planFile, plan)
}

func sanitizeName(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-' || c == '_' || c == '.':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// serviceHostEnvKey turns "helixagent-postgres" / "postgres" into
// "SVC_POSTGRES_HOST" matching the existing internal/config override
// mechanism.
func serviceHostEnvKey(serviceName string) string {
	// Strip the "helixagent-" prefix if present.
	name := serviceName
	if len(name) > len("helixagent-") &&
		name[:len("helixagent-")] == "helixagent-" {
		name = name[len("helixagent-"):]
	}
	out := []byte("SVC_")
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c-32)
		case c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	out = append(out, []byte("_HOST")...)
	return string(out)
}

// hostAddress looks up the registered Address for a host by name.
// Returns "localhost" if the host is unknown (defensive default).
func hostAddress(adapter *containeradapter.Adapter, hostName string) string {
	for _, h := range adapter.ListHosts() {
		if h.Name == hostName {
			if h.Address != "" {
				return h.Address
			}
			return h.Name
		}
	}
	return "localhost"
}

// copyFileSimple copies a file's contents from src to dst.
func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
