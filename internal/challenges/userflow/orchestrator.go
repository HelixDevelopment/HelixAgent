package userflow

import (
	"context"
	"fmt"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/registry"
	"digital.vasic.challenges/pkg/runner"
	uf "digital.vasic.challenges/pkg/userflow"
)

// Orchestrator registers and executes all HelixAgent user flow
// challenges. It uses the Challenges module's runner for
// dependency ordering, parallel execution, and reporting.
type Orchestrator struct {
	registry registry.Registry
	runner   runner.Runner
	adapter  uf.APIAdapter
	baseURL  string
}

// NewOrchestrator creates an Orchestrator for HelixAgent user
// flow testing. The baseURL should point to the running
// HelixAgent server (e.g., "http://localhost:8100").
func NewOrchestrator(baseURL string) (*Orchestrator, error) {
	reg := registry.NewRegistry()
	adapter := uf.NewHTTPAPIAdapter(baseURL)

	o := &Orchestrator{
		registry: reg,
		adapter:  adapter,
		baseURL:  baseURL,
	}

	if err := o.registerChallenges(); err != nil {
		return nil, fmt.Errorf("userflow: register challenges: %w", err)
	}

	r := runner.NewRunner(
		runner.WithRegistry(reg),
	)
	o.runner = r

	return o, nil
}

// registerChallenges registers all HelixAgent user flow
// challenges with their dependency graph.
func (o *Orchestrator) registerChallenges() error {
	healthDep := []challenge.ID{"helix-health-check"}
	providerDep := []challenge.ID{
		"helix-provider-discovery",
	}
	completionDep := []challenge.ID{
		"helix-chat-completion",
	}
	embeddingsDep := []challenge.ID{
		"helix-embeddings",
	}

	reg := func(c challenge.Challenge) error {
		if err := o.registry.Register(c); err != nil {
			return fmt.Errorf(
				"register %s: %w", c.ID(), err,
			)
		}
		return nil
	}

	challenges := []challenge.Challenge{
		NewHealthCheckChallenge(o.adapter),
		NewFeatureFlagsChallenge(o.adapter),
		NewProviderDiscoveryChallenge(
			o.adapter, healthDep,
		),
		NewMonitoringChallenge(
			o.adapter, healthDep,
		),
		NewFormattersChallenge(
			o.adapter, healthDep,
		),
		NewChatCompletionChallenge(
			o.adapter, providerDep,
		),
		NewStreamingCompletionChallenge(
			o.adapter, completionDep,
		),
		NewEmbeddingsChallenge(
			o.adapter, providerDep,
		),
		NewDebateChallenge(
			o.adapter, completionDep,
		),
		NewMCPChallenge(o.adapter, healthDep),
		NewRAGChallenge(
			o.adapter, embeddingsDep,
		),
		NewAuthenticationChallenge(
			o.adapter, healthDep,
		),
		NewErrorHandlingChallenge(
			o.adapter, healthDep,
		),
		NewConcurrentUsersChallenge(
			o.adapter, healthDep,
		),
		NewMultiTurnConversationChallenge(
			o.adapter, completionDep,
		),
		NewToolCallingChallenge(
			o.adapter, completionDep,
		),
		NewProviderFailoverChallenge(
			o.adapter, providerDep,
		),
		NewWebSocketStreamingChallenge(
			o.adapter, healthDep,
		),
		NewGRPCServiceChallenge(
			o.adapter, healthDep,
		),
		NewRateLimitingChallenge(
			o.adapter, healthDep,
		),
		NewPaginationChallenge(
			o.adapter, healthDep,
		),
		NewFullSystemChallenge(o.adapter),
	}

	for _, c := range challenges {
		if err := reg(c); err != nil {
			return err
		}
	}
	return nil
}

// RunAll executes all registered challenges in dependency
// order.
func (o *Orchestrator) RunAll(
	ctx context.Context,
) ([]*challenge.Result, error) {
	config := &challenge.Config{
		ResultsDir: "results/userflow",
		LogsDir:    "logs/userflow",
		Verbose:    true,
	}
	return o.runner.RunAll(ctx, config)
}

// RunByID executes a single challenge by its ID.
func (o *Orchestrator) RunByID(
	ctx context.Context, id string,
) (*challenge.Result, error) {
	config := &challenge.Config{
		ChallengeID: challenge.ID(id),
		ResultsDir:  "results/userflow",
		LogsDir:     "logs/userflow",
		Verbose:     true,
	}
	return o.runner.Run(ctx, challenge.ID(id), config)
}

// ChallengeCount returns the number of registered challenges.
func (o *Orchestrator) ChallengeCount() int {
	return o.registry.Count()
}

// ListChallenges returns all registered challenge IDs.
func (o *Orchestrator) ListChallenges() []string {
	challenges := o.registry.List()
	ids := make([]string, len(challenges))
	for i, c := range challenges {
		ids[i] = string(c.ID())
	}
	return ids
}

// Challenges returns all registered challenges for use by
// the main orchestrator.
func (o *Orchestrator) Challenges() []challenge.Challenge {
	return o.registry.List()
}

// Summary returns a summary string of the orchestrator state.
func (o *Orchestrator) Summary() string {
	return fmt.Sprintf(
		"HelixAgent UserFlow Orchestrator: %d challenges "+
			"registered, base URL: %s",
		o.ChallengeCount(), o.baseURL,
	)
}
