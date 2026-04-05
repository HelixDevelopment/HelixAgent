package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- VerifierErrorResponse JSON round-trip -----------------------------

func TestVerifierErrorResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input VerifierErrorResponse
	}{
		{
			name:  "simple_error",
			input: VerifierErrorResponse{Error: "provider not found"},
		},
		{
			name:  "empty_error",
			input: VerifierErrorResponse{Error: ""},
		},
		{
			name:  "long_error_message",
			input: VerifierErrorResponse{Error: "provider verification failed: timeout after 30s waiting for response from openai API endpoint https://api.openai.com/v1/chat/completions"},
		},
		{
			name:  "error_with_special_characters",
			input: VerifierErrorResponse{Error: "invalid config: key \"api_key\" contains <forbidden> chars & symbols"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)

			var decoded VerifierErrorResponse
			require.NoError(t, json.Unmarshal(data, &decoded))
			assert.Equal(t, tt.input, decoded)
		})
	}
}

// --- JSON field name verification --------------------------------------

func TestVerifierErrorResponse_JSONFieldName(t *testing.T) {
	t.Parallel()
	resp := VerifierErrorResponse{Error: "test error"}
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	// Verify the JSON field is named "error" (from the json tag)
	val, ok := raw["error"]
	assert.True(t, ok, "expected 'error' key in JSON output")
	assert.Equal(t, "test error", val)

	// Verify no unexpected fields
	assert.Len(t, raw, 1, "VerifierErrorResponse should have exactly 1 JSON field")
}

// --- Unmarshal from raw JSON -------------------------------------------

func TestVerifierErrorResponse_UnmarshalFromRawJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		jsonStr  string
		expected VerifierErrorResponse
		wantErr  bool
	}{
		{
			name:     "valid_json",
			jsonStr:  `{"error":"something went wrong"}`,
			expected: VerifierErrorResponse{Error: "something went wrong"},
		},
		{
			name:     "extra_fields_ignored",
			jsonStr:  `{"error":"oops","extra":"field"}`,
			expected: VerifierErrorResponse{Error: "oops"},
		},
		{
			name:     "empty_object",
			jsonStr:  `{}`,
			expected: VerifierErrorResponse{Error: ""},
		},
		{
			name:    "invalid_json",
			jsonStr: `{bad`,
			wantErr: true,
		},
		{
			name:     "null_error_value",
			jsonStr:  `{"error":null}`,
			expected: VerifierErrorResponse{Error: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			var resp VerifierErrorResponse
			err := json.Unmarshal([]byte(tt.jsonStr), &resp)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, resp)
		})
	}
}

// --- Zero value --------------------------------------------------------

func TestVerifierErrorResponse_ZeroValue(t *testing.T) {
	t.Parallel()
	var resp VerifierErrorResponse
	assert.Equal(t, "", resp.Error)

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded VerifierErrorResponse
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, resp, decoded)
}
