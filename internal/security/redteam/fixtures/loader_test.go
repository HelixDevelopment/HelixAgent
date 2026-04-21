package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadByClass_Jailbreak_LoadsSuccessfully asserts that the jailbreak
// fixture file parses cleanly. The fixtures: [] list is currently empty
// pending corpus ingestion — a follow-up task populates it. Every
// fixture that does appear must satisfy the shape invariants below.
func TestLoadByClass_Jailbreak_LoadsSuccessfully(t *testing.T) {
	t.Parallel()

	got, err := LoadByClass(AttackClassJailbreak)
	require.NoError(t, err)

	for _, f := range got {
		assert.NotEmpty(t, f.ID)
		assert.Equal(t, AttackClassJailbreak, f.AttackClass)
		assert.NotEmpty(t, f.Prompt)
		assert.NotEmpty(t, f.ExpectedGuardrailTrigger)
		assert.Contains(t, []string{"low", "medium", "high"}, f.Severity)
	}
}

// TestLoadAll_LoadsEverySupportedClass asserts that every YAML under the
// fixtures/ embed tree parses. Empty fixture lists are acceptable until
// the corpus ingestion task populates them.
func TestLoadAll_LoadsEverySupportedClass(t *testing.T) {
	t.Parallel()

	loaded, err := LoadAll()
	require.NoError(t, err)

	// Every supported attack class must have a file present — the loader
	// returns an entry (possibly empty slice) per file, keyed by class.
	for _, class := range SupportedAttackClasses() {
		_, ok := loaded[class]
		assert.Truef(t, ok, "attack class %q is missing a fixture file", class)
	}
}

func TestLoadByClass_UnknownClass_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := LoadByClass("totally-not-a-class")
	require.Error(t, err)
}
