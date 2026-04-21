# Red-Team Fixtures — Defensive Use Only

These YAML fixtures are adversarial prompt samples used by
`DeepTeamRedTeamer` and `StandardGuardrailPipeline` to verify
HelixAgent's guardrails block the attack classes they describe.

## Policy

- **Defensive use only.** Never feed a fixture to a live model
  without passing it through `StandardGuardrailPipeline` first.
- **Internal only.** Do not mirror, export, or publish these files.
  Directory is `export-ignore` in `.gitattributes`; `git archive`
  will not include it.
- **No source distribution.** The `source_tag` / `provenance` fields
  cite origin for audit purposes; upstream scaffolds were removed
  from the repo on 2026-04-21.

## Schema (per fixture)

| Field | Purpose |
|-------|---------|
| `id` | Stable identifier `redteam.<class>.<seq>` |
| `prompt` | Adversarial input (text) |
| `expected_guardrail_trigger` | Name of the guardrail detector that must flag this fixture |
| `severity` | `low` / `medium` / `high` |
| `source_tag` | Original upstream file reference (for audit) |
| `provenance` | Corpus lineage tag (e.g., `elder-plinius-v3`) |

## Consumer

`internal/security/redteam/fixtures/loader.go` parses these files;
`DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` replays them and
asserts the guardrail trigger. See `tests/security/redteam_fixtures_test.go`.
