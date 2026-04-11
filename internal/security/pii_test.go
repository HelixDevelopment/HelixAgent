package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexPIIDetector_DetectEmail(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		expected int // number of detections
	}{
		{
			name:     "Single email",
			input:    "Contact me at john@example.com",
			expected: 1,
		},
		{
			name:     "Multiple emails",
			input:    "Send to john@example.com and jane@test.org",
			expected: 2,
		},
		{
			name:     "No email",
			input:    "Hello world",
			expected: 0,
		},
		{
			name:     "Gmail address",
			input:    "My email is user123@gmail.com",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detections, err := detector.Detect(context.Background(), tt.input)
			require.NoError(t, err)

			emailCount := 0
			for _, d := range detections {
				if d.Type == PIITypeEmail {
					emailCount++
				}
			}
			assert.Equal(t, tt.expected, emailCount)
		})
	}
}

func TestRegexPIIDetector_DetectPhone(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "US phone with dashes",
			input:    "Call me at 555-123-4567",
			expected: 1,
		},
		{
			name:     "Phone with parentheses",
			input:    "My number is (555) 123-4567",
			expected: 1,
		},
		{
			name:     "No phone",
			input:    "Hello world",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detections, err := detector.Detect(context.Background(), tt.input)
			require.NoError(t, err)

			phoneCount := 0
			for _, d := range detections {
				if d.Type == PIITypePhone {
					phoneCount++
				}
			}
			assert.Equal(t, tt.expected, phoneCount)
		})
	}
}

func TestRegexPIIDetector_DetectSSN(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "SSN with dashes",
			input:    "SSN: 123-45-6789",
			expected: 1,
		},
		{
			name:     "SSN with dots",
			input:    "SSN is 123.45.6789",
			expected: 1,
		},
		{
			name:     "No SSN",
			input:    "Just some text",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detections, err := detector.Detect(context.Background(), tt.input)
			require.NoError(t, err)

			ssnCount := 0
			for _, d := range detections {
				if d.Type == PIITypeSSN {
					ssnCount++
				}
			}
			assert.Equal(t, tt.expected, ssnCount)
		})
	}
}

func TestRegexPIIDetector_DetectCreditCard(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "Visa card",
			input:    "Card: 4111111111111111",
			expected: 1,
		},
		{
			name:     "MasterCard",
			input:    "Card: 5500000000000004",
			expected: 1,
		},
		{
			name:     "No card",
			input:    "Random numbers 12345",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detections, err := detector.Detect(context.Background(), tt.input)
			require.NoError(t, err)

			ccCount := 0
			for _, d := range detections {
				if d.Type == PIITypeCreditCard {
					ccCount++
				}
			}
			assert.Equal(t, tt.expected, ccCount)
		})
	}
}

func TestRegexPIIDetector_DetectAPIKey(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "SK API key",
			input:    "API key: sk_live_1234567890abcdefghij",
			expected: 1,
		},
		{
			name:     "PK API key",
			input:    "Public key: pk_test_abcdefghij1234567890",
			expected: 1,
		},
		{
			name:     "No API key",
			input:    "Just some text",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detections, err := detector.Detect(context.Background(), tt.input)
			require.NoError(t, err)

			apiKeyCount := 0
			for _, d := range detections {
				if d.Type == PIITypeAPIKey {
					apiKeyCount++
				}
			}
			assert.Equal(t, tt.expected, apiKeyCount)
		})
	}
}

func TestRegexPIIDetector_Mask(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name     string
		input    string
		contains string // string that should be in masked output
	}{
		{
			name:     "Email masking",
			input:    "Email: john@example.com",
			contains: "jo**",
		},
		{
			name:     "Phone masking",
			input:    "Call: 555-123-4567",
			contains: "***-***-4567",
		},
		{
			name:     "SSN masking",
			input:    "SSN: 123-45-6789",
			contains: "***-**-6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			masked, detections, err := detector.Mask(context.Background(), tt.input)
			require.NoError(t, err)
			require.NotEmpty(t, detections)

			assert.Contains(t, masked, tt.contains)
		})
	}
}

func TestRegexPIIDetector_Redact(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	input := "Contact john@example.com at 555-123-4567"
	redacted, detections, err := detector.Redact(context.Background(), input)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(detections), 2)
	assert.Contains(t, redacted, "[email_REDACTED]")
	assert.Contains(t, redacted, "[phone_REDACTED]")
	assert.NotContains(t, redacted, "john@example.com")
	assert.NotContains(t, redacted, "555-123-4567")
}

func TestLuhnValidation(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()

	tests := []struct {
		name   string
		number string
		valid  bool
	}{
		{
			name:   "Valid Visa",
			number: "4111111111111111",
			valid:  true,
		},
		{
			name:   "Valid MasterCard",
			number: "5500000000000004",
			valid:  true,
		},
		{
			name:   "Invalid number",
			number: "1234567890123456",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := detector.validateLuhn(tt.number)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestPIIGuardrail(t *testing.T) {
	t.Parallel()
	detector := NewRegexPIIDetector()
	guardrail := NewPIIGuardrail(detector, GuardrailActionWarn, nil)

	t.Run("No PII", func(t *testing.T) {
		t.Parallel()
		result, err := guardrail.Check(context.Background(), "Hello world", nil)
		require.NoError(t, err)
		assert.False(t, result.Triggered)
	})

	t.Run("Contains PII", func(t *testing.T) {
		t.Parallel()
		result, err := guardrail.Check(context.Background(), "Email: test@example.com", nil)
		require.NoError(t, err)
		assert.True(t, result.Triggered)
		assert.Equal(t, GuardrailActionWarn, result.Action)
	})

	t.Run("Modify action masks PII", func(t *testing.T) {
		t.Parallel()
		modifyGuardrail := NewPIIGuardrail(detector, GuardrailActionModify, nil)
		result, err := modifyGuardrail.Check(context.Background(), "Email: test@example.com", nil)
		require.NoError(t, err)
		assert.True(t, result.Triggered)
		assert.NotEmpty(t, result.ModifiedContent)
		assert.NotContains(t, result.ModifiedContent, "test@example.com")
	})
}

// --- Benchmarks ---

func BenchmarkRegexPIIDetector_Detect(b *testing.B) {
	detector := NewRegexPIIDetector()
	text := "Contact john.doe@example.com or call 555-123-4567. SSN: 123-45-6789. Card: 4111111111111111. IP: 192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.Detect(context.Background(), text)
	}
}

func BenchmarkRegexPIIDetector_Mask(b *testing.B) {
	detector := NewRegexPIIDetector()
	text := "Please contact user@gmail.com for further details."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Mask(context.Background(), text)
	}
}

func BenchmarkMaskString(b *testing.B) {
	inputs := []string{"john.doe@example.com", "555-123-4567", "4111111111111111", "sk_live_abcdefghijklmnop"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := inputs[i%len(inputs)]
		_ = maskString(s, 2)
	}
}

func BenchmarkRegexPIIDetector_Redact(b *testing.B) {
	detector := NewRegexPIIDetector()
	text := "Contact john.doe@example.com or call 555-123-4567. SSN: 123-45-6789."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Redact(context.Background(), text)
	}
}

func BenchmarkValidateLuhn(b *testing.B) {
	detector := NewRegexPIIDetector()
	numbers := []string{"4111111111111111", "5500000000000004", "1234567890123456", "378282246310005"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detector.validateLuhn(numbers[i%len(numbers)])
	}
}

func BenchmarkPIIGuardrail_Check(b *testing.B) {
	detector := NewRegexPIIDetector()
	guardrail := NewPIIGuardrail(detector, GuardrailActionWarn, nil)

	inputs := []string{
		"Hello world, no PII here",
		"Email: test@example.com",
		"Call me at 555-123-4567",
		"SSN: 123-45-6789 and Card: 4111111111111111",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = guardrail.Check(context.Background(), inputs[i%len(inputs)], nil)
	}
}
