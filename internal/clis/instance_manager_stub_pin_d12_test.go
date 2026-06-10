package clis

import (
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-11 + D-12 STUB-BLUFF PIN GUARDS — GREEN POLARITY (§11.4.115 / §11.4.135)
//
// HISTORY (the bluff these guards catch): the remaining InstanceManager
// type-specific execute* dispatch methods (executeKiro … executeHelixAgent,
// 43 methods) USED to return a TEMPLATED LITERAL map
// ({"status":"executed","message":"<Agent> execution completed"}) WITHOUT ever
// exec-ing anything — a stub bluff per BLUFF-003 / CONST-035: the dispatch
// claimed "executed" while running nothing.
//
// FIX (SP4-cont D-12): split by binary-confidence (the §11.4.6 honest split):
//   - 3 methods with a CONFIRMED non-interactive CLI binary + flags
//     (executeQwenCoder, executeGitHubCopilot, executeGeminiAssist) now resolve
//     their agent's real CLI binary (resolveAgentBinary) and exec it with the
//     documented non-interactive flags (§11.4.99) via runCLIAgent. Absent binary
//     → honest error, never fake-success — proven by fake-binary injection.
//   - 40 methods with NO confirmed standalone headless CLI (IDE extensions /
//     hosted-web agents / raw model names / a project-owned binary whose name is
//     unverified) now return an HONEST "no non-interactive CLI" error instead of
//     a fabricated "<Agent> execution completed" success. Removing the bluff is
//     the constitutionally-correct state (honest error > fake success, BLUFF-003)
//     where no real exec form can be verified (§11.4.6 no-guessing).
//
// FIX (SP4-cont D-11): IsAgentTypeAvailable USED to be a hard-coded enum
// allowlist with a "For now, allow all types" marker that returned true for
// agents whose execute methods were stubs. It now performs a REAL per-type
// exec.LookPath check (resolveAgentBinary, honoring HELIX_AGENT_BIN_<TYPE>) via
// the shared command table — availability iff a real resolvable binary exists.
// ---------------------------------------------------------------------------

// d12RealExecCases are the 3 D-12 conversions with a confirmed non-interactive
// CLI binary — they really exec, so the full marker-present GREEN guard applies.
func d12RealExecCases(mgr *InstanceManager) []struct {
	name   string
	typ    CLIAgentType
	envKey string
	bluff  string
	fn     func(*AgentInstance, interface{}) (interface{}, error)
} {
	return []struct {
		name   string
		typ    CLIAgentType
		envKey string
		bluff  string
		fn     func(*AgentInstance, interface{}) (interface{}, error)
	}{
		{"qwencoder", TypeQwenCoder, "HELIX_AGENT_BIN_QWENCODER", "Qwen Coder execution completed", mgr.executeQwenCoder},
		{"github_copilot", TypeGitHubCopilot, "HELIX_AGENT_BIN_GITHUB_COPILOT", "GitHub Copilot execution completed", mgr.executeGitHubCopilot},
		{"gemini_assist", TypeGeminiAssist, "HELIX_AGENT_BIN_GEMINI_ASSIST", "Gemini Assist execution completed", mgr.executeGeminiAssist},
	}
}

// d12HonestErrorCases are the 40 D-12 conversions with NO confirmed headless CLI
// — post-fix each returns an honest error, never the bluff literal, never a
// fabricated success.
func d12HonestErrorCases(mgr *InstanceManager) []struct {
	name  string
	typ   CLIAgentType
	bluff string
	fn    func(*AgentInstance, interface{}) (interface{}, error)
} {
	return []struct {
		name  string
		typ   CLIAgentType
		bluff string
		fn    func(*AgentInstance, interface{}) (interface{}, error)
	}{
		{"kiro", TypeKiro, "Kiro execution completed", mgr.executeKiro},
		{"continue", TypeContinue, "Continue.dev execution completed", mgr.executeContinue},
		{"supermaven", TypeSupermaven, "Supermaven execution completed", mgr.executeSupermaven},
		{"cursor", TypeCursor, "Cursor execution completed", mgr.executeCursor},
		{"windsurf", TypeWindsurf, "Windsurf execution completed", mgr.executeWindsurf},
		{"augment", TypeAugment, "Augment execution completed", mgr.executeAugment},
		{"sourcegraph", TypeSourcegraph, "Sourcegraph execution completed", mgr.executeSourcegraph},
		{"codeium", TypeCodeium, "Codeium execution completed", mgr.executeCodeium},
		{"tabnine", TypeTabnine, "Tabnine execution completed", mgr.executeTabnine},
		{"codegpt", TypeCodeGPT, "CodeGPT execution completed", mgr.executeCodeGPT},
		{"twin", TypeTwin, "Twin execution completed", mgr.executeTwin},
		{"devin", TypeDevin, "Devin execution completed", mgr.executeDevin},
		{"devika", TypeDevika, "Devika execution completed", mgr.executeDevika},
		{"swe_agent", TypeSWEAgent, "SWE Agent execution completed", mgr.executeSWEAgent},
		{"gpt_pilot", TypeGPTPilot, "GPT Pilot execution completed", mgr.executeGPTPilot},
		{"metamorph", TypeMetamorph, "Metamorph execution completed", mgr.executeMetamorph},
		{"junie", TypeJunie, "Junie execution completed", mgr.executeJunie},
		{"amazon_q", TypeAmazonQ, "Amazon Q execution completed", mgr.executeAmazonQ},
		{"jetbrains_ai", TypeJetBrainsAI, "JetBrains AI execution completed", mgr.executeJetBrainsAI},
		{"codegemma", TypeCodeGemma, "CodeGemma execution completed", mgr.executeCodeGemma},
		{"starcoder", TypeStarCoder, "StarCoder execution completed", mgr.executeStarCoder},
		{"mistralcode", TypeMistralCode, "Mistral Code execution completed", mgr.executeMistralCode},
		{"codey", TypeCodey, "Codey execution completed", mgr.executeCodey},
		{"llama_code", TypeLlamaCode, "Llama Code execution completed", mgr.executeLlamaCode},
		{"deepseek_coder", TypeDeepSeekCoder, "DeepSeek Coder execution completed", mgr.executeDeepSeekCoder},
		{"wizard_coder", TypeWizardCoder, "WizardCoder execution completed", mgr.executeWizardCoder},
		{"phind", TypePhind, "Phind execution completed", mgr.executePhind},
		{"cody", TypeCody, "Cody execution completed", mgr.executeCody},
		{"cursorsh", TypeCursorSh, "Cursor.sh execution completed", mgr.executeCursorSh},
		{"trae", TypeTrae, "Trae execution completed", mgr.executeTrae},
		{"blackbox", TypeBlackbox, "Blackbox execution completed", mgr.executeBlackbox},
		{"lovable", TypeLovable, "Lovable execution completed", mgr.executeLovable},
		{"v0", TypeV0, "V0 execution completed", mgr.executeV0},
		{"tempo", TypeTempo, "Tempo execution completed", mgr.executeTempo},
		{"bolt", TypeBolt, "Bolt execution completed", mgr.executeBolt},
		{"replit_agent", TypeReplitAgent, "Replit Agent execution completed", mgr.executeReplitAgent},
		{"idx", TypeIDX, "IDX execution completed", mgr.executeIDX},
		{"firebase_studio", TypeFirebaseStudio, "Firebase Studio execution completed", mgr.executeFirebaseStudio},
		{"cascade", TypeCascade, "Cascade execution completed", mgr.executeCascade},
		{"helixagent", TypeHelixAgent, "HelixAgent execution completed", mgr.executeHelixAgent},
	}
}

// TestD12_RealExec_ExecsRealBinary is the standing GREEN regression guard
// (§11.4.135) for the 3 confirmed-binary D-12 conversions: with a fake binary
// injected the dispatch surfaces the binary's REAL stdout marker, never the
// templated literal, and forwards the prompt.
func TestD12_RealExec_ExecsRealBinary(t *testing.T) {
	const marker = "FAKE_AGENT_RAN_d12a"
	bin := writeFakeAgentBin(t, marker)
	mgr := &InstanceManager{}

	for _, tc := range d12RealExecCases(mgr) {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, bin)
			inst := &AgentInstance{ID: "pin-d12-" + tc.name, Type: tc.typ}
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				t.Fatalf("dispatch returned error with fake binary injected: %v", err)
			}
			msg := resultMessage(t, res)
			// (a) The dispatch must surface the fake binary's REAL stdout marker.
			if !strings.Contains(msg, marker) {
				t.Fatalf("D12 REGRESSION: %s dispatch did NOT exec the agent binary — "+
					"its stdout marker %q is absent from %q (BLUFF-003 reintroduced?).", tc.name, marker, msg)
			}
			// (b) The templated "<Agent> execution completed" literal is NEVER returned.
			if msg == tc.bluff {
				t.Fatalf("D12 REGRESSION: %s dispatch returned the templated literal %q "+
					"instead of real process output (BLUFF-003 reintroduced).", tc.name, msg)
			}
			// (c) The prompt was forwarded to the binary.
			if !strings.Contains(msg, "print exactly: 42") {
				t.Fatalf("D12 REGRESSION: %s dispatch did not forward the prompt to the binary (got %q).", tc.name, msg)
			}
		})
	}
}

// TestD12_RealExec_AbsentBinaryIsHonestError proves that with NO agent binary
// available each confirmed-binary D-12 dispatch returns an honest error.
func TestD12_RealExec_AbsentBinaryIsHonestError(t *testing.T) {
	mgr := &InstanceManager{}
	missing := t.TempDir() + "/does-not-exist-agent"

	for _, tc := range d12RealExecCases(mgr) {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, missing)
			inst := &AgentInstance{ID: "pin-d12-absent-" + tc.name, Type: tc.typ}
			if _, err := tc.fn(inst, map[string]interface{}{"prompt": "x"}); err == nil {
				t.Fatalf("D12 BLUFF: %s dispatch returned success with NO agent binary available — "+
					"must return an honest error, never a fabricated template.", tc.name)
			}
		})
	}
}

// TestD12_HonestError_NoFakeSuccess is the standing GREEN regression guard
// (§11.4.135) for the 40 no-headless-CLI D-12 conversions: each MUST return an
// honest error and MUST NEVER return the fabricated "<Agent> execution completed"
// literal.
func TestD12_HonestError_NoFakeSuccess(t *testing.T) {
	mgr := &InstanceManager{}

	for _, tc := range d12HonestErrorCases(mgr) {
		t.Run(tc.name, func(t *testing.T) {
			inst := &AgentInstance{ID: "pin-d12-honest-" + tc.name, Type: tc.typ}
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			// (a) An honest error is mandatory — no fabricated success.
			if err == nil {
				t.Fatalf("D12 BLUFF: %s dispatch returned success with no headless CLI — "+
					"must return an honest error, never a fabricated %q template.", tc.name, tc.bluff)
			}
			// (b) If a result map is returned alongside the error, it must NEVER carry
			// the templated bluff literal.
			if res != nil {
				if m, ok := res.(map[string]string); ok && m["message"] == tc.bluff {
					t.Fatalf("D12 BLUFF: %s dispatch returned the templated literal %q (BLUFF-003).", tc.name, tc.bluff)
				}
			}
			// (c) The error text itself must not be the bluff literal masquerading as an error.
			if err.Error() == tc.bluff {
				t.Fatalf("D12 BLUFF: %s error text is the bluff literal %q.", tc.name, tc.bluff)
			}
		})
	}
}

// TestD12_StubsAreBluffs is the §11.4.115 RED-on-broken-artifact reproduction of
// the historical D-12 bluff, runnable ONLY under PIN_STUB_BLUFF=1. On the pre-fix
// stub artifact each method returned its "<Agent> execution completed" literal →
// FAILs (defect genuinely present). On the fixed artifact the standing GREEN
// guards above are the regression guard.
func TestD12_StubsAreBluffs(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: ATM-SP4-D12 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1. Standing GREEN guards are " +
			"TestD12_RealExec_ExecsRealBinary + TestD12_HonestError_NoFakeSuccess.")
	}
	mgr := &InstanceManager{}

	// Real-exec class: on the stub these returned the literal with NO binary on
	// PATH and NO error. Post-fix they return an honest error (no binary) → bluff
	// gone.
	for _, tc := range d12RealExecCases(mgr) {
		t.Run("realexec/"+tc.name, func(t *testing.T) {
			inst := &AgentInstance{ID: "pin-d12-red-" + tc.name, Type: tc.typ}
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				return // fixed artifact: honest error, bluff gone
			}
			if msg := resultMessage(t, res); msg == tc.bluff {
				t.Fatalf("D12 BLUFF PINNED: %s dispatch returned the templated literal %q without "+
					"exec-ing a real binary (BLUFF-003).", tc.name, msg)
			}
		})
	}

	// Honest-error class: on the stub these returned the literal with NO error.
	// Post-fix they return an honest error → bluff gone.
	for _, tc := range d12HonestErrorCases(mgr) {
		t.Run("honest/"+tc.name, func(t *testing.T) {
			inst := &AgentInstance{ID: "pin-d12-red-" + tc.name, Type: tc.typ}
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				return // fixed artifact: honest error, bluff gone
			}
			if m, ok := res.(map[string]string); ok && m["message"] == tc.bluff {
				t.Fatalf("D12 BLUFF PINNED: %s dispatch returned the templated literal %q without "+
					"any real execution (BLUFF-003).", tc.name, m["message"])
			}
		})
	}
}

// TestD11_IsAgentTypeAvailable_RealLookPath is the standing GREEN regression
// guard (§11.4.135) for D-11: availability is a REAL exec.LookPath check via the
// shared command table, honoring the HELIX_AGENT_BIN_<TYPE> override — NOT a
// hard-coded enum allowlist.
func TestD11_IsAgentTypeAvailable_RealLookPath(t *testing.T) {
	const marker = "FAKE_AGENT_RAN_d11"
	bin := writeFakeAgentBin(t, marker)
	mgr := &InstanceManager{}

	// (a) With a fake binary injected for a real-CLI agent type, it is available.
	t.Setenv("HELIX_AGENT_BIN_AIDER", bin)
	if !mgr.IsAgentTypeAvailable(TypeAider) {
		t.Fatalf("D11 REGRESSION: TypeAider reported unavailable with a real binary injected — "+
			"IsAgentTypeAvailable is not performing a real exec.LookPath check.")
	}

	// (b) A no-headless-CLI agent type (empty command in the table) is NEVER
	// available — the old allowlist returned true for TypeKiro/TypeContinue/
	// TypeHelixAgent (stubs that could not actually run); the real check must not.
	for _, typ := range []CLIAgentType{TypeKiro, TypeContinue, TypeHelixAgent, TypeCursor, TypeDevin} {
		if mgr.IsAgentTypeAvailable(typ) {
			t.Fatalf("D11 BLUFF: %s reported available though it has no resolvable CLI binary — "+
				"the old \"allow all types\" allowlist was reintroduced.", typ)
		}
	}

	// (c) A real-CLI agent with NO binary on PATH and NO override is unavailable.
	t.Setenv("HELIX_AGENT_BIN_QWENCODER", t.TempDir()+"/nope")
	if mgr.IsAgentTypeAvailable(TypeQwenCoder) {
		t.Fatalf("D11 BLUFF: TypeQwenCoder reported available with a non-existent binary override.")
	}
}

// TestD11_AllowAllTypesBluff is the §11.4.115 RED-on-broken-artifact reproduction
// of the historical D-11 "allow all types" allowlist bluff, runnable ONLY under
// PIN_STUB_BLUFF=1. On the pre-fix artifact IsAgentTypeAvailable returned true
// for stub-only agents (TypeKiro/TypeContinue/TypeHelixAgent) → FAILs. On the
// fixed artifact the real exec.LookPath check returns false for them.
func TestD11_AllowAllTypesBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: ATM-SP4-D11 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1. Standing GREEN guard is " +
			"TestD11_IsAgentTypeAvailable_RealLookPath.")
	}
	mgr := &InstanceManager{}
	// On the stub these were hard-coded true regardless of any real binary.
	for _, typ := range []CLIAgentType{TypeKiro, TypeContinue, TypeHelixAgent} {
		if mgr.IsAgentTypeAvailable(typ) {
			t.Fatalf("D11 BLUFF PINNED: %s reported available via the \"allow all types\" "+
				"hard-coded allowlist with no real binary present.", typ)
		}
	}
}
