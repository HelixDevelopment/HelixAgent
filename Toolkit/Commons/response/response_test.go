package response

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestJSONParser_ParseJSON(t *testing.T) {
	parser := &JSONParser{}

	// Create test response
	data := map[string]string{"message": "hello", "status": "ok"}
	jsonData, _ := json.Marshal(data)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(jsonData)),
	}

	var result map[string]string
	err := parser.ParseJSON(resp, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result["message"] != "hello" {
		t.Errorf("Expected message 'hello', got %s", result["message"])
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %s", result["status"])
	}
}

func TestJSONParser_ParseJSONFromBytes(t *testing.T) {
	parser := &JSONParser{}

	data := map[string]int{"count": 42, "value": 100}
	jsonData, _ := json.Marshal(data)

	var result map[string]int
	err := parser.ParseJSONFromBytes(jsonData, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result["count"] != 42 {
		t.Errorf("Expected count 42, got %d", result["count"])
	}

	if result["value"] != 100 {
		t.Errorf("Expected value 100, got %d", result["value"])
	}
}

func TestJSONParser_ParseJSONFromBytes_Invalid(t *testing.T) {
	parser := &JSONParser{}

	invalidJSON := []byte(`{"invalid": json}`)

	var result map[string]string
	err := parser.ParseJSONFromBytes(invalidJSON, &result)

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestStreamingParser_ParseStream(t *testing.T) {
	dataReceived := []string{}
	var parseError error

	parser := NewStreamingParser(
		func(data []byte) error {
			dataReceived = append(dataReceived, string(data))
			return nil
		},
		func(err error) {
			parseError = err
		},
	)

	streamData := "data: {\"message\": \"hello\"}\n\ndata: {\"message\": \"world\"}\n\ndata: [DONE]\n"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(streamData))),
	}

	err := parser.ParseStream(resp)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if parseError != nil {
		t.Errorf("Expected no parse error, got %v", parseError)
	}

	expected := []string{"{\"message\": \"hello\"}", "{\"message\": \"world\"}"}
	if len(dataReceived) != len(expected) {
		t.Fatalf("Expected %d data chunks, got %d", len(expected), len(dataReceived))
	}

	for i, expectedData := range expected {
		if dataReceived[i] != expectedData {
			t.Errorf("Expected data[%d] '%s', got '%s'", i, expectedData, dataReceived[i])
		}
	}
}

func TestStreamingParser_ParseStream_WithError(t *testing.T) {
	var parseError error

	parser := NewStreamingParser(
		func(data []byte) error {
			return io.EOF // Simulate processing error
		},
		func(err error) {
			parseError = err
		},
	)

	streamData := "data: {\"message\": \"test\"}\n\n"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(streamData))),
	}

	err := parser.ParseStream(resp)

	if err == nil {
		t.Error("Expected error from data processing")
	}

	if parseError != io.EOF {
		t.Errorf("Expected parse error io.EOF, got %v", parseError)
	}
}

func TestErrorDetector_DetectError_Success(t *testing.T) {
	detector := &ErrorDetector{}

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("success"))),
	}

	err := detector.DetectError(resp)

	if err != nil {
		t.Errorf("Expected no error for 200 status, got %v", err)
	}
}

func TestErrorDetector_DetectError_JSONError(t *testing.T) {
	detector := &ErrorDetector{}

	errorData := map[string]map[string]string{
		"error": {
			"type":    "invalid_request",
			"message": "bad parameter",
			"code":    "invalid_param",
		},
	}
	jsonData, _ := json.Marshal(errorData)

	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}

	err := detector.DetectError(resp)

	if err == nil {
		t.Error("Expected error for 400 status")
	}

	expectedMsg := "API error [invalid_request]: bad parameter"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestErrorDetector_DetectError_GenericError(t *testing.T) {
	detector := &ErrorDetector{}

	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(bytes.NewReader([]byte("internal server error"))),
	}

	err := detector.DetectError(resp)

	if err == nil {
		t.Error("Expected error for 500 status")
	}

	expectedMsg := "HTTP 500: internal server error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestResponseValidator_Validate(t *testing.T) {
	validator := NewResponseValidator("field1", "field2", "field3")

	// Test valid response
	validResp := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	}

	err := validator.Validate(validResp)
	if err != nil {
		t.Errorf("Expected no error for valid response, got %v", err)
	}

	// Test missing field
	invalidResp := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		// field3 is missing
	}

	err = validator.Validate(invalidResp)
	if err == nil {
		t.Error("Expected error for missing field")
	}

	expectedMsg := "required field 'field3' is missing from response"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestPaginationParser_ParsePaginated(t *testing.T) {
	hasNextPage := func(resp map[string]interface{}) bool {
		if next, ok := resp["next"].(string); ok && next != "" {
			return true
		}
		return false
	}

	getNextURL := func(resp map[string]interface{}) string {
		if next, ok := resp["next"].(string); ok {
			return next
		}
		return ""
	}

	parser := NewPaginationParser(hasNextPage, getNextURL)

	responseData := map[string]interface{}{
		"data": []map[string]string{
			{"id": "1", "name": "item1"},
			{"id": "2", "name": "item2"},
		},
		"next": "https://api.example.com?page=2",
	}

	jsonData, _ := json.Marshal(responseData)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(jsonData)),
	}

	var result []map[string]string
	hasNext, nextURL, err := parser.ParsePaginated(resp, &result)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !hasNext {
		t.Error("Expected hasNext to be true")
	}

	if nextURL != "https://api.example.com?page=2" {
		t.Errorf("Expected nextURL 'https://api.example.com?page=2', got %s", nextURL)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result))
	}

	if result[0]["name"] != "item1" {
		t.Errorf("Expected first item name 'item1', got %s", result[0]["name"])
	}
}

func TestChunkedParser_ParseChunked(t *testing.T) {
	chunks := [][]byte{}

	parser := NewChunkedParser(10, func(chunk []byte) error {
		chunks = append(chunks, append([]byte(nil), chunk...)) // copy chunk
		return nil
	})

	data := []byte("Hello, World! This is a test message.")
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(data)),
	}

	err := parser.ParseChunked(resp)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have received chunks
	if len(chunks) == 0 {
		t.Error("Expected to receive chunks")
	}

	// Concatenate chunks and check total data
	var received []byte
	for _, chunk := range chunks {
		received = append(received, chunk...)
	}

	if string(received) != string(data) {
		t.Errorf("Expected received data '%s', got '%s'", string(data), string(received))
	}
}

func TestResponseBuilder_BuildChatResponse(t *testing.T) {
	builder := &ResponseBuilder{}

	data := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"content": "Hello!"}},
		},
	}

	result, err := builder.BuildChatResponse(data)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// For now, it returns data as-is
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestResponseBuilder_BuildEmbeddingResponse(t *testing.T) {
	builder := &ResponseBuilder{}

	data := map[string]interface{}{
		"data": []map[string]interface{}{
			{"embedding": []float64{0.1, 0.2, 0.3}},
		},
	}

	result, err := builder.BuildEmbeddingResponse(data)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// For now, it returns data as-is
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestResponseBuilder_SanitizeResponse(t *testing.T) {
	builder := &ResponseBuilder{}

	input := map[string]interface{}{
		"message": "Hello",
		"token":   "secret-token",
	}

	result := builder.SanitizeResponse(input)

	// For now, it returns as-is
	if result == nil {
		t.Error("Expected non-nil result")
	}
}
