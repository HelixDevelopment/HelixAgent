// Package tools provides MCP tools for checkpoints
package tools

// CheckpointTools returns checkpoint-related MCP tools
func CheckpointTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "checkpoint_create",
			Description: "Create a workspace checkpoint",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Checkpoint name",
					},
					"description": map[string]interface{}{
						"type": "string",
					},
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "checkpoint_restore",
			Description: "Restore workspace to checkpoint",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"checkpoint_id": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"checkpoint_id"},
			},
		},
		{
			Name:        "checkpoint_list",
			Description: "List all checkpoints",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "checkpoint_delete",
			Description: "Delete a checkpoint",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"checkpoint_id": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"checkpoint_id"},
			},
		},
	}
}
