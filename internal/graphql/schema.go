// Package graphql provides a GraphQL API for HelixAgent.
package graphql

import (
	"github.com/graphql-go/graphql"

	"dev.helix.agent/internal/graphql/resolvers"
)

// Schema is the GraphQL schema for HelixAgent.
var Schema graphql.Schema

// healthStatusType is the GraphQL type for HealthStatus.
var healthStatusType = graphql.NewObject(graphql.ObjectConfig{
	Name: "HealthStatus",
	Fields: graphql.Fields{
		"status":        &graphql.Field{Type: graphql.String},
		"latency_ms":    &graphql.Field{Type: graphql.Int},
		"last_check":    &graphql.Field{Type: graphql.DateTime},
		"error_message": &graphql.Field{Type: graphql.String},
	},
})

// capabilitiesType is the GraphQL type for Capabilities.
var capabilitiesType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Capabilities",
	Fields: graphql.Fields{
		"chat":             &graphql.Field{Type: graphql.Boolean},
		"completions":      &graphql.Field{Type: graphql.Boolean},
		"embeddings":       &graphql.Field{Type: graphql.Boolean},
		"vision":           &graphql.Field{Type: graphql.Boolean},
		"tool_use":         &graphql.Field{Type: graphql.Boolean},
		"streaming":        &graphql.Field{Type: graphql.Boolean},
		"function_calling": &graphql.Field{Type: graphql.Boolean},
	},
})

// modelType is the GraphQL type for Model.
var modelType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Model",
	Fields: graphql.Fields{
		"id":                &graphql.Field{Type: graphql.String},
		"name":              &graphql.Field{Type: graphql.String},
		"provider_id":       &graphql.Field{Type: graphql.String},
		"version":           &graphql.Field{Type: graphql.String},
		"context_window":    &graphql.Field{Type: graphql.Int},
		"max_tokens":        &graphql.Field{Type: graphql.Int},
		"supports_tools":    &graphql.Field{Type: graphql.Boolean},
		"supports_vision":   &graphql.Field{Type: graphql.Boolean},
		"supports_streaming": &graphql.Field{Type: graphql.Boolean},
		"score":             &graphql.Field{Type: graphql.Float},
		"rank":              &graphql.Field{Type: graphql.Int},
		"created_at":        &graphql.Field{Type: graphql.DateTime},
	},
})

// providerType is the GraphQL type for Provider.
var providerType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Provider",
	Fields: graphql.Fields{
		"id":            &graphql.Field{Type: graphql.String},
		"name":          &graphql.Field{Type: graphql.String},
		"type":          &graphql.Field{Type: graphql.String},
		"status":        &graphql.Field{Type: graphql.String},
		"score":         &graphql.Field{Type: graphql.Float},
		"models":        &graphql.Field{Type: graphql.NewList(modelType)},
		"health_status": &graphql.Field{Type: healthStatusType},
		"capabilities":  &graphql.Field{Type: capabilitiesType},
		"created_at":    &graphql.Field{Type: graphql.DateTime},
		"updated_at":    &graphql.Field{Type: graphql.DateTime},
	},
})

// responseType is the GraphQL type for Response.
var responseType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Response",
	Fields: graphql.Fields{
		"participant_id": &graphql.Field{Type: graphql.String},
		"content":        &graphql.Field{Type: graphql.String},
		"confidence":     &graphql.Field{Type: graphql.Float},
		"token_count":    &graphql.Field{Type: graphql.Int},
		"latency_ms":     &graphql.Field{Type: graphql.Int},
		"created_at":     &graphql.Field{Type: graphql.DateTime},
	},
})

// debateRoundType is the GraphQL type for DebateRound.
var debateRoundType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DebateRound",
	Fields: graphql.Fields{
		"id":           &graphql.Field{Type: graphql.String},
		"debate_id":    &graphql.Field{Type: graphql.String},
		"round_number": &graphql.Field{Type: graphql.Int},
		"responses":    &graphql.Field{Type: graphql.NewList(responseType)},
		"summary":      &graphql.Field{Type: graphql.String},
		"created_at":   &graphql.Field{Type: graphql.DateTime},
	},
})

// participantType is the GraphQL type for Participant.
var participantType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Participant",
	Fields: graphql.Fields{
		"id":          &graphql.Field{Type: graphql.String},
		"provider_id": &graphql.Field{Type: graphql.String},
		"model_id":    &graphql.Field{Type: graphql.String},
		"position":    &graphql.Field{Type: graphql.String},
		"role":        &graphql.Field{Type: graphql.String},
		"score":       &graphql.Field{Type: graphql.Float},
	},
})

// debateType is the GraphQL type for Debate.
var debateType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Debate",
	Fields: graphql.Fields{
		"id":           &graphql.Field{Type: graphql.String},
		"topic":        &graphql.Field{Type: graphql.String},
		"status":       &graphql.Field{Type: graphql.String},
		"participants": &graphql.Field{Type: graphql.NewList(participantType)},
		"rounds":       &graphql.Field{Type: graphql.NewList(debateRoundType)},
		"conclusion":   &graphql.Field{Type: graphql.String},
		"confidence":   &graphql.Field{Type: graphql.Float},
		"created_at":   &graphql.Field{Type: graphql.DateTime},
		"updated_at":   &graphql.Field{Type: graphql.DateTime},
		"completed_at": &graphql.Field{Type: graphql.DateTime},
	},
})

// taskType is the GraphQL type for Task.
var taskType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Task",
	Fields: graphql.Fields{
		"id":           &graphql.Field{Type: graphql.String},
		"type":         &graphql.Field{Type: graphql.String},
		"status":       &graphql.Field{Type: graphql.String},
		"priority":     &graphql.Field{Type: graphql.Int},
		"progress":     &graphql.Field{Type: graphql.Int},
		"result":       &graphql.Field{Type: graphql.String},
		"error":        &graphql.Field{Type: graphql.String},
		"created_at":   &graphql.Field{Type: graphql.DateTime},
		"started_at":   &graphql.Field{Type: graphql.DateTime},
		"completed_at": &graphql.Field{Type: graphql.DateTime},
	},
})

// verificationResultsType is the GraphQL type for VerificationResults.
var verificationResultsType = graphql.NewObject(graphql.ObjectConfig{
	Name: "VerificationResults",
	Fields: graphql.Fields{
		"total_providers":    &graphql.Field{Type: graphql.Int},
		"verified_providers": &graphql.Field{Type: graphql.Int},
		"total_models":       &graphql.Field{Type: graphql.Int},
		"verified_models":    &graphql.Field{Type: graphql.Int},
		"overall_score":      &graphql.Field{Type: graphql.Float},
		"last_verified":      &graphql.Field{Type: graphql.DateTime},
	},
})

// providerScoreType is the GraphQL type for ProviderScore.
var providerScoreType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ProviderScore",
	Fields: graphql.Fields{
		"provider_id":        &graphql.Field{Type: graphql.String},
		"provider_name":      &graphql.Field{Type: graphql.String},
		"overall_score":      &graphql.Field{Type: graphql.Float},
		"response_speed":     &graphql.Field{Type: graphql.Float},
		"model_efficiency":   &graphql.Field{Type: graphql.Float},
		"cost_effectiveness": &graphql.Field{Type: graphql.Float},
		"capability":         &graphql.Field{Type: graphql.Float},
		"recency":            &graphql.Field{Type: graphql.Float},
	},
})

// Input types

// providerFilterInput is the GraphQL input type for ProviderFilter.
var providerFilterInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "ProviderFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"status":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"type":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"min_score": &graphql.InputObjectFieldConfig{Type: graphql.Float},
		"max_score": &graphql.InputObjectFieldConfig{Type: graphql.Float},
	},
})

// debateFilterInput is the GraphQL input type for DebateFilter.
var debateFilterInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "DebateFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"status": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// taskFilterInput is the GraphQL input type for TaskFilter.
var taskFilterInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "TaskFilter",
	Fields: graphql.InputObjectConfigFieldMap{
		"status": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"type":   &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// createDebateInput is the GraphQL input type for CreateDebateInput.
var createDebateInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CreateDebateInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"topic":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"participants": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		"round_count":  &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

// debateResponseInput is the GraphQL input type for DebateResponseInput.
var debateResponseInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "DebateResponseInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"debate_id":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"participant_id": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"content":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// createTaskInput is the GraphQL input type for CreateTaskInput.
var createTaskInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CreateTaskInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"priority": &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

// QueryType is the root query type.
var QueryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		// Provider queries
		"providers": &graphql.Field{
			Type:        graphql.NewList(providerType),
			Description: "Get all providers with optional filtering",
			Args: graphql.FieldConfigArgument{
				"filter": &graphql.ArgumentConfig{Type: providerFilterInput},
			},
			Resolve: ResolveProviders,
		},
		"provider": &graphql.Field{
			Type:        providerType,
			Description: "Get a specific provider by ID",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: ResolveProvider,
		},

		// Debate queries
		"debates": &graphql.Field{
			Type:        graphql.NewList(debateType),
			Description: "Get all debates with optional filtering",
			Args: graphql.FieldConfigArgument{
				"filter": &graphql.ArgumentConfig{Type: debateFilterInput},
			},
			Resolve: ResolveDebates,
		},
		"debate": &graphql.Field{
			Type:        debateType,
			Description: "Get a specific debate by ID",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: ResolveDebate,
		},

		// Task queries
		"tasks": &graphql.Field{
			Type:        graphql.NewList(taskType),
			Description: "Get all tasks with optional filtering",
			Args: graphql.FieldConfigArgument{
				"filter": &graphql.ArgumentConfig{Type: taskFilterInput},
			},
			Resolve: ResolveTasks,
		},
		"task": &graphql.Field{
			Type:        taskType,
			Description: "Get a specific task by ID",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: ResolveTask,
		},

		// Verification queries
		"verificationResults": &graphql.Field{
			Type:        verificationResultsType,
			Description: "Get verification results",
			Resolve:     ResolveVerificationResults,
		},
		"providerScores": &graphql.Field{
			Type:        graphql.NewList(providerScoreType),
			Description: "Get provider scores",
			Resolve:     ResolveProviderScores,
		},
	},
})

// MutationType is the root mutation type.
var MutationType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Mutation",
	Fields: graphql.Fields{
		// Debate mutations
		"createDebate": &graphql.Field{
			Type:        debateType,
			Description: "Create a new debate",
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(createDebateInput)},
			},
			Resolve: ResolveCreateDebate,
		},
		"submitDebateResponse": &graphql.Field{
			Type:        debateRoundType,
			Description: "Submit a response to a debate round",
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(debateResponseInput)},
			},
			Resolve: ResolveSubmitDebateResponse,
		},

		// Task mutations
		"createTask": &graphql.Field{
			Type:        taskType,
			Description: "Create a new task",
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(createTaskInput)},
			},
			Resolve: ResolveCreateTask,
		},
		"cancelTask": &graphql.Field{
			Type:        taskType,
			Description: "Cancel a task",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: ResolveCancelTask,
		},

		// Provider mutations
		"refreshProvider": &graphql.Field{
			Type:        providerType,
			Description: "Refresh a provider's status",
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: ResolveRefreshProvider,
		},
	},
})

// Resolver functions - delegating to the resolvers package implementations

// ResolveProviders resolves the providers query.
var ResolveProviders graphql.FieldResolveFn = resolvers.ResolveProviders

// ResolveProvider resolves the provider query.
var ResolveProvider graphql.FieldResolveFn = resolvers.ResolveProvider

// ResolveDebates resolves the debates query.
var ResolveDebates graphql.FieldResolveFn = resolvers.ResolveDebates

// ResolveDebate resolves the debate query.
var ResolveDebate graphql.FieldResolveFn = resolvers.ResolveDebate

// ResolveTasks resolves the tasks query.
var ResolveTasks graphql.FieldResolveFn = resolvers.ResolveTasks

// ResolveTask resolves the task query.
var ResolveTask graphql.FieldResolveFn = resolvers.ResolveTask

// ResolveVerificationResults resolves the verificationResults query.
var ResolveVerificationResults graphql.FieldResolveFn = resolvers.ResolveVerificationResults

// ResolveProviderScores resolves the providerScores query.
var ResolveProviderScores graphql.FieldResolveFn = resolvers.ResolveProviderScores

// ResolveCreateDebate resolves the createDebate mutation.
var ResolveCreateDebate graphql.FieldResolveFn = resolvers.ResolveCreateDebate

// ResolveSubmitDebateResponse resolves the submitDebateResponse mutation.
var ResolveSubmitDebateResponse graphql.FieldResolveFn = resolvers.ResolveSubmitDebateResponse

// ResolveCreateTask resolves the createTask mutation.
var ResolveCreateTask graphql.FieldResolveFn = resolvers.ResolveCreateTask

// ResolveCancelTask resolves the cancelTask mutation.
var ResolveCancelTask graphql.FieldResolveFn = resolvers.ResolveCancelTask

// ResolveRefreshProvider resolves the refreshProvider mutation.
var ResolveRefreshProvider graphql.FieldResolveFn = resolvers.ResolveRefreshProvider

// InitSchema initializes the GraphQL schema.
func InitSchema() error {
	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:    QueryType,
		Mutation: MutationType,
	})
	return err
}

// ExecuteQuery executes a GraphQL query.
func ExecuteQuery(query string, variables map[string]interface{}) *graphql.Result {
	return graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query,
		VariableValues: variables,
	})
}
