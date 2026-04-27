package placement

import (
	"context"
	"fmt"
	"sort"

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

	sched := scheduler.NewScheduler(hostManager, nil, opts...)

	hostServices := make(map[string][]string)
	var allDecisions []scheduler.PlacementDecision

	for _, gid := range groupIDs {
		members := groups[gid]
		// Aggregate group-level requirement: max of any member.
		repr := aggregateRequirements(gid, members)
		decision, err := sched.Schedule(ctx, repr)
		if err != nil {
			return nil, fmt.Errorf("schedule group %s: %w", gid, err)
		}
		// Record one decision per actual member so the per-service
		// audit trail is complete.
		for _, m := range members {
			allDecisions = append(allDecisions, scheduler.PlacementDecision{
				Requirement: m,
				HostName:    decision.HostName,
				Score:       decision.Score,
				Reason:      decision.Reason,
			})
			hostServices[decision.HostName] = append(
				hostServices[decision.HostName], m.Name,
			)
		}
	}

	// Stable per-host service order.
	hostNames := make([]string, 0, len(hostServices))
	for h := range hostServices {
		hostNames = append(hostNames, h)
	}
	sort.Strings(hostNames)

	plan := &Plan{Decisions: allDecisions}
	for _, h := range hostNames {
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
