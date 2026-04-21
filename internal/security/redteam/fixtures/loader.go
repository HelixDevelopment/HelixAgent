// Package fixtures provides defensive-use-only red-team attack fixtures
// consumed by DeepTeamRedTeamer.RunFixtureSuite. Fixtures are replayed
// through StandardGuardrailPipeline to assert that HelixAgent's
// guardrails block the attack classes they describe. They are never
// executed against a live model without a guardrail pipeline in front.
package fixtures

import (
	"embed"
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// AttackClass is the typed identifier for a red-team attack category.
// Values correspond exactly to the `attack_class:` field inside each
// fixture YAML, and to a stem name of a file under this directory.
type AttackClass string

// Supported attack classes. Each corresponds to exactly one
// `<name>.yaml` file embedded below.
const (
	AttackClassJailbreak              AttackClass = "jailbreak"
	AttackClassAbliterationProbe      AttackClass = "abliteration_probe"
	AttackClassFilterBypass           AttackClass = "filter_bypass"
	AttackClassStegoMutation          AttackClass = "stego_mutation"
	AttackClassGeneticSeed            AttackClass = "genetic_seed"
	AttackClassSystemPromptExtraction AttackClass = "system_prompt_extraction"
	AttackClassRoleReversal           AttackClass = "role_reversal"
)

// SupportedAttackClasses returns the canonical list of attack classes
// this loader recognises. The list is the single source of truth for
// both the loader and its consumers (tests, challenges, redteam).
func SupportedAttackClasses() []AttackClass {
	return []AttackClass{
		AttackClassJailbreak,
		AttackClassAbliterationProbe,
		AttackClassFilterBypass,
		AttackClassStegoMutation,
		AttackClassGeneticSeed,
		AttackClassSystemPromptExtraction,
		AttackClassRoleReversal,
	}
}

// Fixture is a single defensive red-team fixture: a prompt known to be
// an attack, plus the guardrail the pipeline is expected to trigger and
// the severity the guardrail should flag.
type Fixture struct {
	ID                       string      `yaml:"id"`
	AttackClass              AttackClass `yaml:"attack_class"`
	Prompt                   string      `yaml:"prompt"`
	ExpectedGuardrailTrigger string      `yaml:"expected_guardrail_trigger"`
	Severity                 string      `yaml:"severity"`
	Source                   string      `yaml:"source,omitempty"`
	Notes                    string      `yaml:"notes,omitempty"`
}

// fixtureFile is the on-disk schema. The top-level attack_class
// serves as a default for per-fixture entries that omit it.
type fixtureFile struct {
	AttackClass AttackClass `yaml:"attack_class"`
	Version     int         `yaml:"version"`
	DateAdded   string      `yaml:"date_added"`
	Fixtures    []Fixture   `yaml:"fixtures"`
}

//go:embed *.yaml
var fixtureFS embed.FS

// LoadByClass returns all fixtures declared for the given attack class.
// Returns an error if the class is not supported or the YAML cannot be
// parsed. An empty slice is a valid return when the fixture file is
// present but its `fixtures:` list is empty (pending corpus ingestion).
func LoadByClass(class AttackClass) ([]Fixture, error) {
	if !isSupported(class) {
		return nil, fmt.Errorf("fixtures: unsupported attack class %q", class)
	}

	filename := string(class) + ".yaml"
	data, err := fixtureFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("fixtures: read %s: %w", filename, err)
	}

	var parsed fixtureFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("fixtures: parse %s: %w", filename, err)
	}

	// Propagate the file-level attack_class down to any fixture that
	// omitted it, so downstream consumers never see an empty class.
	for i := range parsed.Fixtures {
		if parsed.Fixtures[i].AttackClass == "" {
			parsed.Fixtures[i].AttackClass = parsed.AttackClass
		}
	}

	return parsed.Fixtures, nil
}

// LoadAll returns every fixture file keyed by AttackClass. Files whose
// `fixtures:` list is empty are still represented (with an empty slice
// value) so consumers can detect that the class is wired but unfilled.
func LoadAll() (map[AttackClass][]Fixture, error) {
	entries, err := fixtureFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("fixtures: list embed root: %w", err)
	}

	result := make(map[AttackClass][]Fixture, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if path.Ext(name) != ".yaml" {
			continue
		}
		stem := AttackClass(strings.TrimSuffix(name, ".yaml"))
		if !isSupported(stem) {
			// Unknown file in the embed tree — surface as an error so
			// a typo or forgotten taxonomy entry fails loudly.
			return nil, fmt.Errorf(
				"fixtures: file %s declares unknown attack class %q", name, stem,
			)
		}
		fixtures, err := LoadByClass(stem)
		if err != nil {
			return nil, err
		}
		result[stem] = fixtures
	}
	return result, nil
}

func isSupported(class AttackClass) bool {
	for _, c := range SupportedAttackClasses() {
		if c == class {
			return true
		}
	}
	return false
}
