package placement

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// HostAssignment lists every service the planner placed onto a host,
// the absolute path of the per-host filtered compose file (after
// EmitPerHostCompose), and the source compose file.
type HostAssignment struct {
	HostName    string
	ServiceList []string
	ComposeFile string // source
	OutFile     string // per-host filtered (set by Plan after writing)
}

// Plan is the output of PlanCompose: one HostAssignment per host that
// was selected by the scheduler. Hosts that received no services are
// omitted.
type Plan struct {
	Assignments []HostAssignment
	// Decisions is the underlying scheduler output for diagnostics.
	Decisions []scheduler.PlacementDecision
}

// PlanCompose runs the full placement flow for a single compose file:
//
//  1. Parse the compose file → ContainerRequirements (with co-location
//     groups derived from depends_on).
//  2. Group requirements by their CoLocationLabel: any two services in
//     the same group are scheduled as ONE atomic unit (placed on the
//     same host).
//  3. For each group, call the scheduler to pick a host using the
//     supplied strategy. The scheduler probes hosts via the
//     HostManager.
//  4. Aggregate per-host service lists into a Plan.
//
// The scheduler's StrategyAffinity is honored when present; otherwise
// the strategy from `opts` (typically StrategyResourceAware) is used.
//
// If `profile` is non-empty, only services with that profile (or no
// profiles list) are considered; matches Compose semantics.
//
// The caller is expected to follow up with EmitPerHostCompose to
// produce the per-host filtered compose files.
func PlanCompose(
	ctx context.Context,
	composeFile, profile string,
	hostManager remote.HostManager,
	opts ...scheduler.Option,
) (*Plan, error) {
	reqs, err := ParseCompose(composeFile, profile)
	if err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}
	if len(reqs) == 0 {
		return &Plan{}, nil
	}

	// Group requirements by CoLocationLabel. Each group becomes one
	// scheduling unit — pick a representative requirement (the one
	// with the highest mem demand so the scheduler has the strictest
	// constraint to satisfy).
	groups := make(map[string][]scheduler.ContainerRequirements)
	for _, r := range reqs {
		gid := r.Labels[CoLocationLabel]
		if gid == "" {
			gid = r.Name // ungrouped service is its own group
		}
		groups[gid] = append(groups[gid], r)
	}

	// Deterministic group iteration order (stable plans across reboots).
	groupIDs := make([]string, 0, len(groups))
	for g := range groups {
		groupIDs = append(groupIDs, g)
	}
	sort.Strings(groupIDs)

	// Capability-aware placement: per group, evaluate every
	// registered remote host using ScoreHost (capability.go).
	// HARD constraints (gpu/runtime/arch/memory-fit) gate
	// eligibility; SOFT preferences (storage/memory/network class)
	// add to score; load penalty (placement count × weight) breaks
	// ties toward less-loaded hosts. Local is excluded by
	// construction — we only consider ListHosts().
	//
	// `opts` is accepted for forward compatibility (e.g. when the
	// Containers scheduler grows a "no-local" option that would let
	// us delegate again) but not consumed today.
	_ = opts

	hosts := hostManager.ListHosts()
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no remote hosts registered")
	}

	// Build HostCapabilities for every registered host. If a
	// CapabilityProber is configured on the planner (set via
	// SetProber from the adapter at boot), probe live; otherwise
	// fall back to label-only capabilities (workable for tests and
	// when SSH is briefly unavailable — the scorer degrades to load
	// balancing across eligible hosts).
	caps := buildHostCapabilities(ctx, hosts)

	hostByName := make(map[string]*HostCapabilities, len(caps))
	for _, c := range caps {
		hostByName[c.Name] = c
	}

	hostServices := make(map[string][]string)
	var allDecisions []scheduler.PlacementDecision

	for _, gid := range groupIDs {
		members := groups[gid]
		groupReq := aggregateRequirementsForCapability(gid, members)

		picked, scoreResults := PickBestHost(groupReq, caps)
		if picked == "" {
			// No eligible host. Record the failure on every member
			// so the persisted plan shows operators why nothing was
			// scheduled.
			reason := summarizeIneligibility(scoreResults)
			for _, m := range members {
				allDecisions = append(allDecisions, scheduler.PlacementDecision{
					Requirement: m,
					HostName:    "",
					Score:       0,
					Reason:      reason,
				})
			}
			continue
		}

		// Update load on the picked host so subsequent groups see
		// the new placement when scoring.
		if c, ok := hostByName[picked]; ok {
			c.PlacementCount += len(members)
		}

		// Build a single reason string from the winning ScoreResult
		// for audit. (Other hosts' results omitted from the
		// per-member decision; the full breakdown is logged at INFO
		// once per group in adapter.RemoteComposeUp.)
		var reason string
		for _, sr := range scoreResults {
			if sr.HostName == picked {
				reason = sr.String()
				break
			}
		}

		for _, m := range members {
			allDecisions = append(allDecisions, scheduler.PlacementDecision{
				Requirement: m,
				HostName:    picked,
				Score:       0,
				Reason:      reason,
			})
			hostServices[picked] = append(hostServices[picked], m.Name)
		}
	}

	// Stable per-host service order.
	emittedHosts := make([]string, 0, len(hostServices))
	for h := range hostServices {
		emittedHosts = append(emittedHosts, h)
	}
	sort.Strings(emittedHosts)

	plan := &Plan{Decisions: allDecisions}
	for _, h := range emittedHosts {
		svcs := hostServices[h]
		sort.Strings(svcs)
		plan.Assignments = append(plan.Assignments, HostAssignment{
			HostName:    h,
			ServiceList: svcs,
			ComposeFile: composeFile,
		})
	}
	return plan, nil
}

// aggregateRequirements produces the strictest constraint across a
// co-location group so the scheduler picks a host that fits the whole
// group, not just one member.
//
// Labels are intentionally LEFT EMPTY here: scheduler.ContainerRequirements.Labels
// are interpreted by the scheduler as REQUIRED host labels (the host
// must carry every label-value to be eligible). The placement.group
// label we attached during ParseCompose has served its purpose
// (grouping members for this aggregation step) and must NOT be passed
// through, otherwise the scheduler filters out every host because no
// host carries our synthetic group label and every Schedule() returns
// an empty HostName. (BUGFIXES.md Issue #52 — initial fix forgot this
// and produced "host \"\" not registered" on every deploy.)
func aggregateRequirements(
	gid string, members []scheduler.ContainerRequirements,
) scheduler.ContainerRequirements {
	out := scheduler.ContainerRequirements{
		Name: "group:" + gid,
	}
	var totalCPU float64
	var totalMem uint64
	for _, m := range members {
		totalCPU += m.CPUCores
		totalMem += m.MemoryMB
		// GPU: any member needing GPU forces the group to require GPU.
		if m.GPU != nil && out.GPU == nil {
			out.GPU = m.GPU
		}
	}
	out.CPUCores = totalCPU
	out.MemoryMB = totalMem
	return out
}

// aggregateRequirementsForCapability produces the placement.Requirement
// fed into capability.ScoreHost. Labels are unioned across members
// using "strictest wins" semantics:
//
//  - require.gpu / require.runtime / require.arch — any member's hard
//    constraint propagates to the whole group (placing on a host that
//    fails any member's constraint would break that member).
//  - prefer.* — strictest preference wins (e.g. one member preferring
//    "high" memory pulls the group toward high-memory hosts even if
//    others ask "medium").
func aggregateRequirementsForCapability(
	gid string, members []scheduler.ContainerRequirements,
) Requirement {
	out := Requirement{
		Name:   "group:" + gid,
		Labels: map[string]string{},
	}
	classRank := map[string]int{
		"low": 1, "medium": 2, "high": 3, "slow": 1, "fast": 3,
	}
	for _, m := range members {
		out.MemoryMB += m.MemoryMB
		out.CPUCores += m.CPUCores
		for k, v := range m.Labels {
			switch k {
			case LabelRequireGPU, LabelRequireRuntime, LabelRequireArch:
				if existing, ok := out.Labels[k]; ok && existing != "" && existing != v {
					// Conflicting hard constraint — pick the
					// stricter one (non-"any" wins; otherwise the
					// existing one).
					if v != "any" && v != "" {
						out.Labels[k] = v
					}
				} else {
					out.Labels[k] = v
				}
			case LabelPreferStorage, LabelPreferMemory, LabelPreferNetwork:
				existing := out.Labels[k]
				if classRank[v] > classRank[existing] {
					out.Labels[k] = v
				}
			}
		}
	}
	return out
}

// summarizeIneligibility builds a single string explaining why no
// host accepted the group. Sorts by host name so the message is
// stable across reboots.
func summarizeIneligibility(results []ScoreResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, r.String())
	}
	return "no eligible host: " + strings.Join(parts, " | ")
}

// buildHostCapabilities returns one HostCapabilities per registered
// remote host. When a CapabilityProber has been configured on the
// package via SetCapabilityProber (called from
// adapter.go::RemoteComposeUp), each host is probed live; otherwise
// a label-only HostCapabilities is returned (workable for tests and
// when the executor is unavailable).
func buildHostCapabilities(ctx context.Context, hosts []remote.RemoteHost) []*HostCapabilities {
	out := make([]*HostCapabilities, 0, len(hosts))
	for _, h := range hosts {
		var caps *HostCapabilities
		if proberMu.RLock(); globalCapabilityProber != nil {
			c, err := globalCapabilityProber.Probe(ctx, h)
			proberMu.RUnlock()
			if err == nil && c != nil {
				caps = c
			}
		} else {
			proberMu.RUnlock()
		}
		if caps == nil {
			// Label-only fallback. The scorer still works — it just
			// can't honor preferences whose host class is unknown.
			caps = &HostCapabilities{
				Name:   h.Name,
				Labels: copyLabels(h.Labels),
			}
			overrideFromHostLabels(caps, h.Labels)
			deriveClassesFromCaps(caps)
		}
		out = append(out, caps)
	}
	return out
}
