module unified-cli-generator

go 1.25.3

require digital.vasic.llmsverifier v0.0.0

// Five levels up, not four: this module sits at
// <repo>/challenges/codebase/go_files/unified_cli_generator, so `../../../..`
// is the helix_agent repo root and the verifier is a SIBLING submodule one
// level above that. The old four-level `LLMsVerifier` path was also stale in
// case — CONST-052 renamed the directory to lowercase snake_case. Either error
// alone breaks the build; both together made it look like a plausible path.
replace digital.vasic.llmsverifier => ../../../../../llms_verifier/llm-verifier
