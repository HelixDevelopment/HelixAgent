package handlers

import (
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/services"
)

// BuildAgenticEnsemble constructs the real, dynamic, subagent-driven
// AgenticEnsemble engine from its existing dependencies and wires it for
// out-of-the-box operation.
//
// Per §11.4.74 (reuse, don't reimplement) this is pure wiring: it
// composes the already-implemented services (DebateService, the
// LLM-backed intent classifier, the execution planner, the verification
// debate, and the provider registry) into the existing
// services.AgenticEnsemble — it does NOT re-create any of the engine.
//
// The engine is built with services.DefaultAgenticEnsembleConfig
// (EnableExecution:true) so actionable / decompose prompts route through
// the 7-stage decompose→execute pipeline by default, with no opt-in flag.
//
// Nil dependencies are tolerated by services.NewAgenticEnsemble (it
// degrades gracefully when a subsystem is unavailable), which keeps the
// helper safe to call early during server boot and from unit tests.
func BuildAgenticEnsemble(
	debateService *services.DebateService,
	intentClassifier *services.LLMIntentClassifier,
	providerRegistry *services.ProviderRegistry,
	logger *logrus.Logger,
) *services.AgenticEnsemble {
	if logger == nil {
		logger = logrus.New()
	}

	cfg := services.DefaultAgenticEnsembleConfig()

	// The intent classifier decides reason-vs-execute mode. The caller may
	// pass nil when its own classifier is gated on a StartupVerifier that
	// is not ready at router-setup time (it is initialised asynchronously
	// after provider verification). Without a classifier the ensemble can
	// NEVER enter execute mode (classifyMode early-returns reason), so the
	// decompose→execute / subagent-spawning path would be dead. Construct
	// one directly from the provider registry — which IS available at wiring
	// time — so the engine's execute path works out-of-the-box regardless of
	// verifier timing.
	if intentClassifier == nil && providerRegistry != nil {
		intentClassifier = services.NewLLMIntentClassifier(providerRegistry, logger)
	}

	// Real LLM-backed task decomposition + result verification. Both
	// constructors only need a logger; they are the genuine engine
	// components, not stand-ins.
	planner := services.NewExecutionPlanner(logger)
	verifier := services.NewVerificationDebate(logger)

	// The iterative tool executor is left nil here: the tool bridge is an
	// optional augmentation and the ensemble's reason/execute paths
	// degrade gracefully without it. Wiring a concrete bridge is a
	// separate, additive enhancement and is not required for the
	// decompose→execute engine to run.
	return services.NewAgenticEnsemble(
		debateService,
		intentClassifier,
		nil, // tool executor (optional augmentation)
		planner,
		verifier,
		providerRegistry,
		cfg,
		logger,
	)
}
