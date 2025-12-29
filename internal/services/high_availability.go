package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolFederation manages cross-protocol communication and data sharing
type ProtocolFederation struct {
	mu            sync.RWMutex
	protocols     map[string]FederatedProtocol
	translators   map[string]*DataTranslator
	eventBus      *EventBus
	subscriptions map[string][]EventSubscription
	logger        *logrus.Logger
}

// FederatedProtocol represents a protocol that can participate in federation
type FederatedProtocol interface {
	Name() string
	HandleFederatedRequest(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error)
	PublishEvent(ctx context.Context, event *ProtocolEvent) error
	GetCapabilities() map[string]interface{}
}

// FederatedRequest represents a cross-protocol request
type FederatedRequest struct {
	ID            string                 `json:"id"`
	Source        string                 `json:"source"`
	Target        string                 `json:"target"`
	Action        string                 `json:"action"`
	Data          map[string]interface{} `json:"data"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlationId,omitempty"`
}

// FederatedResponse represents a cross-protocol response
type FederatedResponse struct {
	ID            string                 `json:"id"`
	Success       bool                   `json:"success"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlationId,omitempty"`
}

// ProtocolEvent represents an event that can be shared across protocols
type ProtocolEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// EventSubscription represents an event subscription
type EventSubscription struct {
	ID        string
	Protocol  string
	EventType string
	Handler   EventHandler
}

// EventHandler defines an event handler function
type EventHandler func(ctx context.Context, event *ProtocolEvent) error

// EventBus manages event distribution
type EventBus struct {
	subscribers map[string][]EventHandler
	mu          sync.RWMutex
	logger      *logrus.Logger
}

// DataTranslator handles data format translation between protocols
type DataTranslator struct {
	SourceProtocol string
	TargetProtocol string
	Translations   map[string]TranslationRule
}

// TranslationRule defines how to translate data between protocols
type TranslationRule struct {
	SourcePath string
	TargetPath string
	Transform  DataTransform
}

// DataTransform defines a data transformation function
type DataTransform func(input interface{}) (interface{}, error)

// NewProtocolFederation creates a new protocol federation manager
func NewProtocolFederation(logger *logrus.Logger) *ProtocolFederation {
	return &ProtocolFederation{
		protocols:     make(map[string]FederatedProtocol),
		translators:   make(map[string]*DataTranslator),
		eventBus:      NewEventBus(logger),
		subscriptions: make(map[string][]EventSubscription),
		logger:        logger,
	}
}

// RegisterProtocol registers a protocol for federation
func (pf *ProtocolFederation) RegisterProtocol(protocol FederatedProtocol) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	name := protocol.Name()
	if _, exists := pf.protocols[name]; exists {
		return fmt.Errorf("protocol %s already registered", name)
	}

	pf.protocols[name] = protocol

	pf.logger.WithField("protocol", name).Info("Protocol registered for federation")
	return nil
}

// UnregisterProtocol removes a protocol from federation
func (pf *ProtocolFederation) UnregisterProtocol(protocolName string) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if _, exists := pf.protocols[protocolName]; !exists {
		return fmt.Errorf("protocol %s not registered", protocolName)
	}

	delete(pf.protocols, protocolName)

	// Clean up subscriptions
	delete(pf.subscriptions, protocolName)

	pf.logger.WithField("protocol", protocolName).Info("Protocol unregistered from federation")
	return nil
}

// SendFederatedRequest sends a request to another protocol
func (pf *ProtocolFederation) SendFederatedRequest(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
	pf.mu.RLock()
	protocol, exists := pf.protocols[request.Target]
	pf.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("target protocol %s not registered", request.Target)
	}

	// Translate data if needed
	translatedRequest := request
	if translator, exists := pf.translators[pf.getTranslatorKey(request.Source, request.Target)]; exists {
		var err error
		translatedRequest, err = pf.translateRequest(translator, request)
		if err != nil {
			return nil, fmt.Errorf("failed to translate request: %w", err)
		}
	}

	response, err := protocol.HandleFederatedRequest(ctx, translatedRequest)
	if err != nil {
		return nil, err
	}

	// Translate response if needed
	if translator, exists := pf.translators[pf.getTranslatorKey(request.Target, request.Source)]; exists {
		var err error
		response, err = pf.translateResponse(translator, response)
		if err != nil {
			pf.logger.WithError(err).Warn("Failed to translate response")
		}
	}

	return response, nil
}

// PublishEvent publishes an event to all subscribed protocols
func (pf *ProtocolFederation) PublishEvent(ctx context.Context, event *ProtocolEvent) error {
	return pf.eventBus.Publish(ctx, event)
}

// SubscribeToEvents subscribes a protocol to events
func (pf *ProtocolFederation) SubscribeToEvents(protocolName, eventType string, handler EventHandler) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if _, exists := pf.protocols[protocolName]; !exists {
		return fmt.Errorf("protocol %s not registered", protocolName)
	}

	subscription := EventSubscription{
		ID:        fmt.Sprintf("%s-%s-%d", protocolName, eventType, time.Now().Unix()),
		Protocol:  protocolName,
		EventType: eventType,
		Handler:   handler,
	}

	pf.subscriptions[protocolName] = append(pf.subscriptions[protocolName], subscription)

	// Subscribe to event bus
	pf.eventBus.Subscribe(eventType, handler)

	pf.logger.WithFields(logrus.Fields{
		"protocol":  protocolName,
		"eventType": eventType,
	}).Info("Protocol subscribed to events")

	return nil
}

// UnsubscribeFromEvents unsubscribes a protocol from events
func (pf *ProtocolFederation) UnsubscribeFromEvents(protocolName, subscriptionID string) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	subscriptions, exists := pf.subscriptions[protocolName]
	if !exists {
		return fmt.Errorf("no subscriptions found for protocol %s", protocolName)
	}

	for i, sub := range subscriptions {
		if sub.ID == subscriptionID {
			pf.subscriptions[protocolName] = append(subscriptions[:i], subscriptions[i+1:]...)
			break
		}
	}

	pf.logger.WithFields(logrus.Fields{
		"protocol":       protocolName,
		"subscriptionId": subscriptionID,
	}).Info("Protocol unsubscribed from events")

	return nil
}

// AddDataTranslator adds a data translator between protocols
func (pf *ProtocolFederation) AddDataTranslator(translator *DataTranslator) error {
	key := pf.getTranslatorKey(translator.SourceProtocol, translator.TargetProtocol)
	pf.translators[key] = translator

	pf.logger.WithFields(logrus.Fields{
		"source": translator.SourceProtocol,
		"target": translator.TargetProtocol,
	}).Info("Data translator added")

	return nil
}

// GetRegisteredProtocols returns all registered protocols
func (pf *ProtocolFederation) GetRegisteredProtocols() []string {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	protocols := make([]string, 0, len(pf.protocols))
	for name := range pf.protocols {
		protocols = append(protocols, name)
	}

	return protocols
}

// GetProtocolCapabilities returns capabilities for a protocol
func (pf *ProtocolFederation) GetProtocolCapabilities(protocolName string) (map[string]interface{}, error) {
	pf.mu.RLock()
	protocol, exists := pf.protocols[protocolName]
	pf.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("protocol %s not registered", protocolName)
	}

	return protocol.GetCapabilities(), nil
}

// BroadcastRequest broadcasts a request to all registered protocols
func (pf *ProtocolFederation) BroadcastRequest(ctx context.Context, action string, data map[string]interface{}) map[string]*FederatedResponse {
	pf.mu.RLock()
	protocols := make(map[string]FederatedProtocol)
	for k, v := range pf.protocols {
		protocols[k] = v
	}
	pf.mu.RUnlock()

	results := make(map[string]*FederatedResponse)
	correlationID := fmt.Sprintf("broadcast-%d", time.Now().Unix())

	for protocolName := range protocols {
		request := &FederatedRequest{
			ID:            fmt.Sprintf("%s-%d", protocolName, time.Now().UnixNano()),
			Source:        "federation",
			Target:        protocolName,
			Action:        action,
			Data:          data,
			Timestamp:     time.Now(),
			CorrelationID: correlationID,
		}

		response, err := pf.SendFederatedRequest(ctx, request)
		if err != nil {
			results[protocolName] = &FederatedResponse{
				ID:            request.ID,
				Success:       false,
				Error:         err.Error(),
				Timestamp:     time.Now(),
				CorrelationID: correlationID,
			}
		} else {
			results[protocolName] = response
		}
	}

	return results
}

// Private methods

func (pf *ProtocolFederation) getTranslatorKey(source, target string) string {
	return fmt.Sprintf("%s-%s", source, target)
}

func (pf *ProtocolFederation) translateRequest(translator *DataTranslator, request *FederatedRequest) (*FederatedRequest, error) {
	translated := &FederatedRequest{
		ID:            request.ID,
		Source:        request.Source,
		Target:        request.Target,
		Action:        request.Action,
		Data:          make(map[string]interface{}),
		Timestamp:     request.Timestamp,
		CorrelationID: request.CorrelationID,
	}

	// Copy original data
	for k, v := range request.Data {
		translated.Data[k] = v
	}

	// Apply translations
	for _, rule := range translator.Translations {
		if value, exists := pf.getNestedValue(request.Data, rule.SourcePath); exists {
			translatedValue, err := rule.Transform(value)
			if err != nil {
				return nil, fmt.Errorf("translation failed for %s: %w", rule.SourcePath, err)
			}
			pf.setNestedValue(translated.Data, rule.TargetPath, translatedValue)
		}
	}

	return translated, nil
}

func (pf *ProtocolFederation) translateResponse(translator *DataTranslator, response *FederatedResponse) (*FederatedResponse, error) {
	translated := &FederatedResponse{
		ID:            response.ID,
		Success:       response.Success,
		Data:          make(map[string]interface{}),
		Error:         response.Error,
		Timestamp:     response.Timestamp,
		CorrelationID: response.CorrelationID,
	}

	// Copy original data
	for k, v := range response.Data {
		translated.Data[k] = v
	}

	// Apply translations
	for _, rule := range translator.Translations {
		if value, exists := pf.getNestedValue(response.Data, rule.SourcePath); exists {
			translatedValue, err := rule.Transform(value)
			if err != nil {
				return nil, fmt.Errorf("translation failed for %s: %w", rule.SourcePath, err)
			}
			pf.setNestedValue(translated.Data, rule.TargetPath, translatedValue)
		}
	}

	return translated, nil
}

func (pf *ProtocolFederation) getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			value, exists := current[key]
			return value, exists
		}

		if nested, ok := current[key].(map[string]interface{}); ok {
			current = nested
		} else {
			return nil, false
		}
	}

	return nil, false
}

func (pf *ProtocolFederation) setNestedValue(data map[string]interface{}, path string, value interface{}) {
	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			return
		}

		if current[key] == nil {
			current[key] = make(map[string]interface{})
		}

		if nested, ok := current[key].(map[string]interface{}); ok {
			current = nested
		} else {
			// Create new nested map
			newMap := make(map[string]interface{})
			current[key] = newMap
			current = newMap
		}
	}
}

// EventBus implementation

// NewEventBus creates a new event bus
func NewEventBus(logger *logrus.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[string][]EventHandler),
		logger:      logger,
	}
}

// Subscribe subscribes to an event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
	eb.logger.WithField("eventType", eventType).Debug("Event subscription added")
}

// Unsubscribe removes a subscription
func (eb *EventBus) Unsubscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	handlers := eb.subscribers[eventType]
	for i, h := range handlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			eb.subscribers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(ctx context.Context, event *ProtocolEvent) error {
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		return nil // No subscribers
	}

	// Publish to all handlers (fire and forget for now)
	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h(ctx, event); err != nil {
				eb.logger.WithError(err).WithField("eventType", event.Type).Error("Event handler failed")
			}
		}(handler)
	}

	eb.logger.WithFields(logrus.Fields{
		"eventType":   event.Type,
		"subscribers": len(handlers),
	}).Debug("Event published")

	return nil
}

// Common data transformations

// IdentityTransform returns the input unchanged
func IdentityTransform(input interface{}) (interface{}, error) {
	return input, nil
}

// StringToIntTransform converts a string to int
func StringToIntTransform(input interface{}) (interface{}, error) {
	if str, ok := input.(string); ok {
		// Simple conversion - in real implementation, use strconv
		switch str {
		case "true":
			return 1, nil
		case "false":
			return 0, nil
		default:
			return 0, fmt.Errorf("cannot convert string to int: %s", str)
		}
	}
	return input, nil
}

// JSONTransform marshals/unmarshals JSON
func JSONTransform(input interface{}) (interface{}, error) {
	if data, err := json.Marshal(input); err != nil {
		return nil, err
	} else {
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

// Protocol-specific federation wrappers

// MCPFederatedProtocol wraps MCP for federation
type MCPFederatedProtocol struct {
	client *MCPClient
}

// NewMCPFederatedProtocol creates a new MCP federation wrapper
func NewMCPFederatedProtocol(client *MCPClient) *MCPFederatedProtocol {
	return &MCPFederatedProtocol{client: client}
}

func (m *MCPFederatedProtocol) Name() string {
	return "mcp"
}

func (m *MCPFederatedProtocol) HandleFederatedRequest(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
	// Convert federated request to MCP tool call
	toolName := request.Action
	servers := m.client.ListServers()

	if len(servers) == 0 {
		return &FederatedResponse{
			ID:      request.ID,
			Success: false,
			Error:   "no MCP servers available",
		}, nil
	}

	serverID := servers[0].ID // Use first available server
	result, err := m.client.CallTool(ctx, serverID, toolName, request.Data)

	response := &FederatedResponse{
		ID:            request.ID,
		Success:       err == nil,
		CorrelationID: request.CorrelationID,
		Timestamp:     time.Now(),
	}

	if err != nil {
		response.Error = err.Error()
	} else {
		response.Data = map[string]interface{}{
			"result": result,
		}
	}

	return response, nil
}

func (m *MCPFederatedProtocol) PublishEvent(ctx context.Context, event *ProtocolEvent) error {
	// MCP doesn't have built-in event publishing
	// This could be implemented as tool calls or notifications
	return nil
}

func (m *MCPFederatedProtocol) GetCapabilities() map[string]interface{} {
	tools, err := m.client.ListTools(context.Background())
	toolCount := 0
	if err == nil {
		toolCount = len(tools)
	}
	return map[string]interface{}{
		"tools":   toolCount,
		"servers": len(m.client.ListServers()),
		"type":    "mcp",
	}
}

// LSPFederatedProtocol wraps LSP for federation
type LSPFederatedProtocol struct {
	client *LSPClient
}

// NewLSPFederatedProtocol creates a new LSP federation wrapper
func NewLSPFederatedProtocol(client *LSPClient) *LSPFederatedProtocol {
	return &LSPFederatedProtocol{client: client}
}

func (l *LSPFederatedProtocol) Name() string {
	return "lsp"
}

func (l *LSPFederatedProtocol) HandleFederatedRequest(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
	response := &FederatedResponse{
		ID:            request.ID,
		Success:       false,
		CorrelationID: request.CorrelationID,
		Timestamp:     time.Now(),
	}

	// LSP operations typically need file context
	// This is a simplified implementation
	switch request.Action {
	case "completion":
		if uri, ok := request.Data["uri"].(string); ok {
			if line, ok := request.Data["line"].(float64); ok {
				if character, ok := request.Data["character"].(float64); ok {
					// Use first available server
					servers := l.client.ListServers()
					if len(servers) > 0 {
						serverID := servers[0].ID
						completion, err := l.client.GetCompletion(ctx, serverID, uri, int(line), int(character))
						if err != nil {
							response.Error = err.Error()
						} else {
							response.Success = true
							response.Data = map[string]interface{}{
								"completion": completion,
							}
						}
					} else {
						response.Error = "no LSP servers available"
					}
				} else {
					response.Error = "missing character parameter"
				}
			} else {
				response.Error = "missing line parameter"
			}
		} else {
			response.Error = "missing uri parameter"
		}
	default:
		response.Error = fmt.Sprintf("unsupported LSP action: %s", request.Action)
	}

	return response, nil
}

func (l *LSPFederatedProtocol) PublishEvent(ctx context.Context, event *ProtocolEvent) error {
	// LSP doesn't have built-in event publishing
	return nil
}

func (l *LSPFederatedProtocol) GetCapabilities() map[string]interface{} {
	servers := l.client.ListServers()
	return map[string]interface{}{
		"servers":   len(servers),
		"type":      "lsp",
		"languages": []string{"go", "python", "typescript", "rust"}, // Common languages
	}
}
