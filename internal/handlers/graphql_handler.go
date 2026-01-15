// Package handlers provides HTTP handlers for HelixAgent endpoints.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/graphql"
	"dev.helix.agent/internal/graphql/resolvers"
	"dev.helix.agent/internal/toon"
)

// GraphQLHandler handles GraphQL requests.
type GraphQLHandler struct {
	logger        *logrus.Logger
	toonEncoder   *toon.Encoder
	toonDecoder   *toon.Decoder
	enableTOON    bool
	initialized   bool
}

// GraphQLRequest represents a GraphQL request.
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message   string        `json:"message"`
	Locations []Location    `json:"locations,omitempty"`
	Path      []interface{} `json:"path,omitempty"`
}

// Location represents an error location.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLHandlerConfig configures the GraphQL handler.
type GraphQLHandlerConfig struct {
	// EnableTOON enables TOON encoding/decoding support.
	EnableTOON bool `json:"enable_toon" yaml:"enable_toon"`
	// TOONCompression sets the TOON compression level.
	TOONCompression toon.CompressionLevel `json:"toon_compression" yaml:"toon_compression"`
}

// DefaultGraphQLHandlerConfig returns default configuration.
func DefaultGraphQLHandlerConfig() *GraphQLHandlerConfig {
	return &GraphQLHandlerConfig{
		EnableTOON:      true,
		TOONCompression: toon.CompressionStandard,
	}
}

// NewGraphQLHandler creates a new GraphQL handler.
func NewGraphQLHandler(logger *logrus.Logger, config *GraphQLHandlerConfig) (*GraphQLHandler, error) {
	if config == nil {
		config = DefaultGraphQLHandlerConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	// Initialize the GraphQL schema
	if err := graphql.InitSchema(); err != nil {
		return nil, err
	}

	// Wire up the resolvers
	graphql.ResolveProviders = resolvers.ResolveProviders
	graphql.ResolveProvider = resolvers.ResolveProvider
	graphql.ResolveDebates = resolvers.ResolveDebates
	graphql.ResolveDebate = resolvers.ResolveDebate
	graphql.ResolveTasks = resolvers.ResolveTasks
	graphql.ResolveTask = resolvers.ResolveTask
	graphql.ResolveVerificationResults = resolvers.ResolveVerificationResults
	graphql.ResolveProviderScores = resolvers.ResolveProviderScores
	graphql.ResolveCreateDebate = resolvers.ResolveCreateDebate
	graphql.ResolveSubmitDebateResponse = resolvers.ResolveSubmitDebateResponse
	graphql.ResolveCreateTask = resolvers.ResolveCreateTask
	graphql.ResolveCancelTask = resolvers.ResolveCancelTask
	graphql.ResolveRefreshProvider = resolvers.ResolveRefreshProvider

	h := &GraphQLHandler{
		logger:      logger,
		enableTOON:  config.EnableTOON,
		initialized: true,
	}

	if config.EnableTOON {
		opts := &toon.EncoderOptions{
			Compression: config.TOONCompression,
		}
		h.toonEncoder = toon.NewEncoder(opts)
		h.toonDecoder = toon.NewDecoder(opts)
	}

	return h, nil
}

// Handle handles a GraphQL request.
func (h *GraphQLHandler) Handle(c *gin.Context) {
	// Check content type for TOON
	contentType := c.GetHeader("Content-Type")
	acceptTOON := c.GetHeader("Accept") == "application/toon+json"
	isTOON := contentType == "application/toon+json"

	// Parse request
	var req GraphQLRequest
	var err error

	if isTOON && h.enableTOON && h.toonDecoder != nil {
		// Decode TOON request
		body, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.JSON(http.StatusBadRequest, GraphQLResponse{
				Errors: []GraphQLError{{Message: "Failed to read request body"}},
			})
			return
		}
		err = h.toonDecoder.Decode(body, &req)
	} else {
		// Standard JSON request
		err = c.ShouldBindJSON(&req)
	}

	if err != nil {
		h.logger.WithError(err).Debug("Failed to parse GraphQL request")
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Invalid request: " + err.Error()}},
		})
		return
	}

	if req.Query == "" {
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Query is required"}},
		})
		return
	}

	// Execute query
	result := graphql.ExecuteQuery(req.Query, req.Variables)

	// Build response
	response := GraphQLResponse{
		Data: result.Data,
	}

	if len(result.Errors) > 0 {
		response.Errors = make([]GraphQLError, len(result.Errors))
		for i, err := range result.Errors {
			response.Errors[i] = GraphQLError{
				Message: err.Message,
			}
			if err.Locations != nil {
				response.Errors[i].Locations = make([]Location, len(err.Locations))
				for j, loc := range err.Locations {
					response.Errors[i].Locations[j] = Location{
						Line:   loc.Line,
						Column: loc.Column,
					}
				}
			}
			if err.Path != nil {
				response.Errors[i].Path = err.Path
			}
		}
	}

	// Return response
	if acceptTOON && h.enableTOON && h.toonEncoder != nil {
		// Encode as TOON
		encoded, encodeErr := h.toonEncoder.Encode(response)
		if encodeErr != nil {
			h.logger.WithError(encodeErr).Warn("Failed to encode TOON response")
			c.JSON(http.StatusOK, response)
			return
		}
		c.Header("Content-Type", "application/toon+json")
		c.Data(http.StatusOK, "application/toon+json", encoded)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// HandleIntrospection handles GraphQL introspection queries.
func (h *GraphQLHandler) HandleIntrospection(c *gin.Context) {
	introspectionQuery := `
		query IntrospectionQuery {
			__schema {
				queryType { name }
				mutationType { name }
				subscriptionType { name }
				types {
					...FullType
				}
				directives {
					name
					description
					locations
					args {
						...InputValue
					}
				}
			}
		}

		fragment FullType on __Type {
			kind
			name
			description
			fields(includeDeprecated: true) {
				name
				description
				args {
					...InputValue
				}
				type {
					...TypeRef
				}
				isDeprecated
				deprecationReason
			}
			inputFields {
				...InputValue
			}
			interfaces {
				...TypeRef
			}
			enumValues(includeDeprecated: true) {
				name
				description
				isDeprecated
				deprecationReason
			}
			possibleTypes {
				...TypeRef
			}
		}

		fragment InputValue on __InputValue {
			name
			description
			type {
				...TypeRef
			}
			defaultValue
		}

		fragment TypeRef on __Type {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
						ofType {
							kind
							name
							ofType {
								kind
								name
								ofType {
									kind
									name
									ofType {
										kind
										name
									}
								}
							}
						}
					}
				}
			}
		}
	`

	result := graphql.ExecuteQuery(introspectionQuery, nil)

	response := GraphQLResponse{
		Data: result.Data,
	}

	if len(result.Errors) > 0 {
		response.Errors = make([]GraphQLError, len(result.Errors))
		for i, err := range result.Errors {
			response.Errors[i] = GraphQLError{Message: err.Message}
		}
	}

	c.JSON(http.StatusOK, response)
}

// HandlePlayground serves a GraphQL playground HTML page.
func (h *GraphQLHandler) HandlePlayground(c *gin.Context) {
	html := `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HelixAgent GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root">
    <style>
      body {
        margin: 0;
        overflow: hidden;
      }
      .loading {
        display: flex;
        justify-content: center;
        align-items: center;
        height: 100vh;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
        color: #172B4D;
      }
    </style>
    <div class="loading">Loading GraphQL Playground...</div>
  </div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/v1/graphql',
        settings: {
          'request.credentials': 'include',
          'editor.theme': 'dark'
        }
      });
    });
  </script>
</body>
</html>
`
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// RegisterRoutes registers GraphQL routes on the router.
func (h *GraphQLHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/v1/graphql", h.Handle)
	router.GET("/v1/graphql", h.HandlePlayground)
	router.GET("/v1/graphql/introspection", h.HandleIntrospection)
}

// RegisterRoutesOnGroup registers GraphQL routes on a router group.
func (h *GraphQLHandler) RegisterRoutesOnGroup(group *gin.RouterGroup) {
	group.POST("/graphql", h.Handle)
	group.GET("/graphql", h.HandlePlayground)
	group.GET("/graphql/introspection", h.HandleIntrospection)
}

// SetResolverContext sets the resolver context for dependency injection.
func (h *GraphQLHandler) SetResolverContext(ctx *resolvers.ResolverContext) {
	resolvers.SetGlobalContext(ctx)
}

// IsInitialized returns whether the handler is properly initialized.
func (h *GraphQLHandler) IsInitialized() bool {
	return h.initialized
}

// Stats returns handler statistics.
type GraphQLStats struct {
	Initialized bool `json:"initialized"`
	TOONEnabled bool `json:"toon_enabled"`
}

// GetStats returns handler statistics.
func (h *GraphQLHandler) GetStats() GraphQLStats {
	return GraphQLStats{
		Initialized: h.initialized,
		TOONEnabled: h.enableTOON,
	}
}

// GraphQLBatchRequest represents a batched GraphQL request.
type GraphQLBatchRequest []GraphQLRequest

// HandleBatch handles batched GraphQL requests.
func (h *GraphQLHandler) HandleBatch(c *gin.Context) {
	var batch GraphQLBatchRequest

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Failed to read request body"}},
		})
		return
	}

	if err := json.Unmarshal(body, &batch); err != nil {
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Invalid batch request: " + err.Error()}},
		})
		return
	}

	if len(batch) == 0 {
		c.JSON(http.StatusBadRequest, GraphQLResponse{
			Errors: []GraphQLError{{Message: "Empty batch request"}},
		})
		return
	}

	// Process each request
	responses := make([]GraphQLResponse, len(batch))
	for i, req := range batch {
		result := graphql.ExecuteQuery(req.Query, req.Variables)

		responses[i] = GraphQLResponse{
			Data: result.Data,
		}

		if len(result.Errors) > 0 {
			responses[i].Errors = make([]GraphQLError, len(result.Errors))
			for j, err := range result.Errors {
				responses[i].Errors[j] = GraphQLError{Message: err.Message}
			}
		}
	}

	c.JSON(http.StatusOK, responses)
}
