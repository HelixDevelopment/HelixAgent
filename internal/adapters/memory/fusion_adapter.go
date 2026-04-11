// Package memory provides the HelixMemory Fusion adapter for HelixAgent.
// This adapter integrates the unified Cognee+Mem0+Letta fusion engine
// as the default memory system for HelixAgent.
package memory

import (
	"context"
	"fmt"
	"log"

	"dev.helix.agent/internal/config"
	helixmem "dev.helix.agent/internal/memory"

	"digital.vasic.helixmemory/pkg/clients/cognee"
	"digital.vasic.helixmemory/pkg/clients/mem0"
	helixcfg "digital.vasic.helixmemory/pkg/config"
	"digital.vasic.helixmemory/pkg/fusion"
	"digital.vasic.helixmemory/pkg/types"
)

// HelixMemoryFusionAdapter provides direct access to the HelixMemory FusionEngine.
// This is the recommended adapter for production use as it provides:
// - Unified access to Cognee, Mem0, and Letta
// - Intelligent routing based on memory type
// - Automatic fallback between systems
// - Result fusion and deduplication
type HelixMemoryFusionAdapter struct {
	engine *fusion.FusionEngine
	config *helixcfg.Config
}

// NewHelixMemoryFusionAdapter creates a new fusion adapter.
// This is the DEFAULT memory implementation for HelixAgent.
func NewHelixMemoryFusionAdapter(cfg *config.Config) (*HelixMemoryFusionAdapter, error) {
	// Load HelixMemory configuration
	hcfg, err := helixcfg.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load HelixMemory config: %w", err)
	}

	// Create fusion engine
	engine, err := fusion.NewFusionEngine(hcfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fusion engine: %w", err)
	}

	adapter := &HelixMemoryFusionAdapter{
		engine: engine,
		config: hcfg,
	}

	// Log active services
	services := hcfg.GetActiveServices()
	log.Printf("[HelixMemory] Fusion engine initialized with services: %v", services)

	return adapter, nil
}

// Add stores a memory in the appropriate system(s).
func (a *HelixMemoryFusionAdapter) Add(ctx context.Context, memory *helixmem.Memory) error {
	entry := a.toHelixMemoryEntry(memory)
	return a.engine.Store(ctx, entry)
}

// Get retrieves a memory by ID.
func (a *HelixMemoryFusionAdapter) Get(ctx context.Context, id string) (*helixmem.Memory, error) {
	// Try each system
	if a.config.HasMem0() {
		client := mem0.NewClient(a.config)
		entry, err := client.Get(ctx, id)
		if err == nil {
			return a.fromHelixMemoryEntry(entry), nil
		}
	}

	if a.config.HasCognee() {
		client := cognee.NewClient(a.config)
		entry, err := client.Get(ctx, id)
		if err == nil {
			return a.fromHelixMemoryEntry(entry), nil
		}
	}

	return nil, fmt.Errorf("memory not found: %s", id)
}

// Update modifies an existing memory.
func (a *HelixMemoryFusionAdapter) Update(ctx context.Context, memory *helixmem.Memory) error {
	entry := a.toHelixMemoryEntry(memory)
	return a.engine.Store(ctx, entry)
}

// Delete removes a memory.
func (a *HelixMemoryFusionAdapter) Delete(ctx context.Context, id string) error {
	return a.engine.Delete(ctx, id)
}

// Search searches across all memory systems.
func (a *HelixMemoryFusionAdapter) Search(ctx context.Context, query string, opts *helixmem.SearchOptions) ([]*helixmem.Memory, error) {
	req := &types.SearchRequest{
		Query:  query,
		TopK:   opts.TopK,
		UserID: opts.UserID,
	}

	if opts.TimeRange != nil {
		req.TimeRange = &types.TimeRange{
			Start: opts.TimeRange.Start,
			End:   opts.TimeRange.End,
		}
	}

	result, err := a.engine.Retrieve(ctx, req)
	if err != nil {
		return nil, err
	}

	memories := make([]*helixmem.Memory, len(result.Entries))
	for i, entry := range result.Entries {
		memories[i] = a.fromHelixMemoryEntry(entry)
	}

	return memories, nil
}

// GetByUser retrieves memories for a user.
func (a *HelixMemoryFusionAdapter) GetByUser(ctx context.Context, userID string, opts *helixmem.ListOptions) ([]*helixmem.Memory, error) {
	result, err := a.engine.GetHistory(ctx, userID, opts.Limit)
	if err != nil {
		return nil, err
	}

	memories := make([]*helixmem.Memory, len(result.Entries))
	for i, entry := range result.Entries {
		memories[i] = a.fromHelixMemoryEntry(entry)
	}

	return memories, nil
}

// GetBySession retrieves memories for a session.
func (a *HelixMemoryFusionAdapter) GetBySession(ctx context.Context, sessionID string) ([]*helixmem.Memory, error) {
	req := &types.SearchRequest{
		Query:     "*",
		SessionID: sessionID,
		TopK:      100,
	}

	result, err := a.engine.Retrieve(ctx, req)
	if err != nil {
		return nil, err
	}

	memories := make([]*helixmem.Memory, len(result.Entries))
	for i, entry := range result.Entries {
		memories[i] = a.fromHelixMemoryEntry(entry)
	}

	return memories, nil
}

// AddEntity adds an entity to the knowledge graph (Cognee).
func (a *HelixMemoryFusionAdapter) AddEntity(ctx context.Context, entity *helixmem.Entity) error {
	// Store as graph memory in Cognee
	entry := &types.MemoryEntry{
		ID:      entity.ID,
		Content: fmt.Sprintf("%s: %s (%s)", entity.Name, entity.Type, entity.Properties),
		Type:    types.MemoryTypeGraph,
		Metadata: map[string]interface{}{
			"entity_name": entity.Name,
			"entity_type": entity.Type,
			"properties":  entity.Properties,
		},
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}

	return a.engine.CreateKnowledgeGraph(ctx, entry.Content, entry.Metadata)
}

// GetEntity retrieves an entity from the knowledge graph.
func (a *HelixMemoryFusionAdapter) GetEntity(ctx context.Context, id string) (*helixmem.Entity, error) {
	// Search in Cognee
	req := &types.SearchRequest{
		Query:  id,
		Filter: map[string]interface{}{"search_type": "GRAPH_COMPLETION"},
		TopK:   1,
	}

	client := cognee.NewClient(a.config)
	result, err := client.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	entry := result.Entries[0]
	return &helixmem.Entity{
		ID:         entry.ID,
		Name:       entry.Metadata["entity_name"].(string),
		Type:       entry.Metadata["entity_type"].(string),
		Properties: entry.Metadata["properties"].(map[string]interface{}),
		CreatedAt:  entry.CreatedAt,
		UpdatedAt:  entry.UpdatedAt,
	}, nil
}

// SearchEntities searches for entities in the knowledge graph.
func (a *HelixMemoryFusionAdapter) SearchEntities(ctx context.Context, query string, limit int) ([]*helixmem.Entity, error) {
	result, err := a.engine.QueryKnowledgeGraph(ctx, query)
	if err != nil {
		return nil, err
	}

	entities := make([]*helixmem.Entity, 0, len(result.Entries))
	for _, entry := range result.Entries {
		if entry.Type == types.MemoryTypeGraph {
			entities = append(entities, &helixmem.Entity{
				ID:        entry.ID,
				Name:      entry.Metadata["entity_name"].(string),
				Type:      entry.Metadata["entity_type"].(string),
				CreatedAt: entry.CreatedAt,
				UpdatedAt: entry.UpdatedAt,
			})
		}
	}

	return entities, nil
}

// AddRelationship adds a relationship between entities.
func (a *HelixMemoryFusionAdapter) AddRelationship(ctx context.Context, rel *helixmem.Relationship) error {
	// Store as graph connection
	content := fmt.Sprintf("%s -[%s]-> %s", rel.SourceID, rel.Type, rel.TargetID)
	return a.engine.CreateKnowledgeGraph(ctx, content, map[string]interface{}{
		"relationship_type": rel.Type,
		"source_id":         rel.SourceID,
		"target_id":         rel.TargetID,
	})
}

// GetRelationships gets relationships for an entity.
func (a *HelixMemoryFusionAdapter) GetRelationships(ctx context.Context, entityID string) ([]*helixmem.Relationship, error) {
	// Query for relationships involving this entity
	req := &types.SearchRequest{
		Query: entityID,
		Filter: map[string]interface{}{
			"search_type": "GRAPH_COMPLETION",
		},
		TopK: 100,
	}

	client := cognee.NewClient(a.config)
	result, err := client.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	relationships := make([]*helixmem.Relationship, 0)
	for _, entry := range result.Entries {
		if relType, ok := entry.Metadata["relationship_type"].(string); ok {
			relationships = append(relationships, &helixmem.Relationship{
				SourceID:  entry.Metadata["source_id"].(string),
				TargetID:  entry.Metadata["target_id"].(string),
				Type:      relType,
				CreatedAt: entry.CreatedAt,
				UpdatedAt: entry.UpdatedAt,
			})
		}
	}

	return relationships, nil
}

// Health checks the health of all memory systems.
func (a *HelixMemoryFusionAdapter) Health(ctx context.Context) map[string]error {
	results := a.engine.HealthCheck(ctx)

	// Convert to string keys
	health := make(map[string]error)
	for source, err := range results {
		health[string(source)] = err
	}

	return health
}

// GetStats returns statistics about the memory systems.
func (a *HelixMemoryFusionAdapter) GetStats() types.FusionStats {
	return a.engine.GetStats()
}

// StoreWithAgent stores a memory and associates it with an agent.
func (a *HelixMemoryFusionAdapter) StoreWithAgent(ctx context.Context, memory *helixmem.Memory, agentID string) error {
	entry := a.toHelixMemoryEntry(memory)
	entry.AgentID = agentID
	return a.engine.StoreWithAgent(ctx, entry, agentID)
}

// RetrieveForAgent retrieves memories for a specific agent.
func (a *HelixMemoryFusionAdapter) RetrieveForAgent(ctx context.Context, query, agentID string) ([]*helixmem.Memory, error) {
	result, err := a.engine.RetrieveForAgent(ctx, query, agentID)
	if err != nil {
		return nil, err
	}

	memories := make([]*helixmem.Memory, len(result.Entries))
	for i, entry := range result.Entries {
		memories[i] = a.fromHelixMemoryEntry(entry)
	}

	return memories, nil
}

// Private helper methods

func (a *HelixMemoryFusionAdapter) toHelixMemoryEntry(m *helixmem.Memory) *types.MemoryEntry {
	memoryType := types.MemoryTypeSemantic
	switch m.Type {
	case helixmem.MemoryTypeEpisodic:
		memoryType = types.MemoryTypeEpisodic
	case helixmem.MemoryTypeProcedural:
		memoryType = types.MemoryTypeProcedural
	default:
		memoryType = types.MemoryTypeSemantic
	}

	metadata := make(map[string]interface{})
	for k, v := range m.Metadata {
		metadata[k] = v
	}
	metadata["category"] = m.Category
	metadata["importance"] = m.Importance

	return &types.MemoryEntry{
		ID:          m.ID,
		Content:     m.Content,
		Type:        memoryType,
		Metadata:    metadata,
		Embedding:   m.Embedding,
		UserID:      m.UserID,
		SessionID:   m.SessionID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		AccessCount: m.AccessCount,
	}
}

func (a *HelixMemoryFusionAdapter) fromHelixMemoryEntry(e *types.MemoryEntry) *helixmem.Memory {
	memoryType := helixmem.MemoryTypeSemantic
	switch e.Type {
	case types.MemoryTypeEpisodic:
		memoryType = helixmem.MemoryTypeEpisodic
	case types.MemoryTypeProcedural:
		memoryType = helixmem.MemoryTypeProcedural
	default:
		memoryType = helixmem.MemoryTypeSemantic
	}

	metadata := make(map[string]interface{})
	for k, v := range e.Metadata {
		metadata[k] = v
	}

	return &helixmem.Memory{
		ID:          e.ID,
		Content:     e.Content,
		Type:        memoryType,
		Metadata:    metadata,
		Embedding:   e.Embedding,
		UserID:      e.UserID,
		SessionID:   e.SessionID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		AccessCount: e.AccessCount,
	}
}

// Ensure HelixMemoryFusionAdapter implements the MemoryStore interface
var _ helixmem.MemoryStore = (*HelixMemoryFusionAdapter)(nil)
