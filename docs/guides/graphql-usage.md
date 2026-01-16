# GraphQL API Usage Guide

## Overview

HelixAgent provides a GraphQL API for flexible data querying and real-time subscriptions. This complements the REST API for use cases that benefit from GraphQL's capabilities.

## Endpoint

```
POST http://localhost:8080/v1/graphql
```

GraphQL Playground (development mode):
```
GET http://localhost:8080/v1/graphql/playground
```

## Authentication

Include your API key in the Authorization header:

```bash
curl -X POST http://localhost:8080/v1/graphql \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"query": "..."}'
```

## Queries

### List Providers

```graphql
query ListProviders {
  providers(filter: { status: "active", minScore: 7.0 }) {
    edges {
      node {
        id
        name
        type
        status
        score
        healthStatus {
          status
          latency
          lastCheck
        }
        capabilities {
          chat
          streaming
          toolUse
          vision
        }
        models {
          id
          name
          contextWindow
          supportsTools
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
    totalCount
  }
}
```

### Get Provider Details

```graphql
query GetProvider($id: ID!) {
  provider(id: $id) {
    id
    name
    score
    models {
      id
      name
      score
      rank
    }
  }
}
```

### List Debates

```graphql
query ListDebates($filter: DebateFilter) {
  debates(filter: $filter, first: 10) {
    edges {
      node {
        id
        topic
        status
        confidence
        participants {
          id
          providerID
          position
          role
        }
        rounds {
          roundNumber
          responses {
            content
            confidence
            latency
          }
        }
        conclusion
        createdAt
        completedAt
      }
    }
  }
}
```

### Get Verification Results

```graphql
query VerificationResults {
  verification {
    totalProviders
    verifiedProviders
    totalModels
    verifiedModels
    overallScore
    lastVerified
    providerScores {
      providerID
      providerName
      overallScore
      responseSpeed
      modelEfficiency
      costEffectiveness
      capability
      recency
    }
  }
}
```

### List Tasks

```graphql
query ListTasks($filter: TaskFilter) {
  tasks(filter: $filter, first: 20) {
    edges {
      node {
        id
        type
        status
        priority
        progress
        result
        error
        createdAt
        startedAt
        completedAt
      }
    }
  }
}
```

## Mutations

### Create Debate

```graphql
mutation CreateDebate($input: CreateDebateInput!) {
  createDebate(input: $input) {
    id
    topic
    status
    participants {
      id
      providerID
    }
    createdAt
  }
}

# Variables
{
  "input": {
    "topic": "Should AI have consciousness?",
    "participants": ["claude", "gemini", "deepseek"],
    "roundCount": 3
  }
}
```

### Create Task

```graphql
mutation CreateTask($input: CreateTaskInput!) {
  createTask(input: $input) {
    id
    type
    status
    priority
    createdAt
  }
}

# Variables
{
  "input": {
    "type": "verification",
    "priority": 1,
    "payload": {
      "provider_id": "openai"
    }
  }
}
```

### Verify Provider

```graphql
mutation VerifyProvider($id: ID!) {
  verifyProvider(id: $id) {
    providerID
    overallScore
    responseSpeed
    capability
  }
}
```

## Subscriptions

### Debate Updates

```graphql
subscription DebateUpdates($debateId: ID!) {
  debateUpdates(debateId: $debateId) {
    debateId
    type
    round {
      roundNumber
      responses {
        participantID
        content
        confidence
      }
    }
    conclusion
    timestamp
  }
}
```

### Task Progress

```graphql
subscription TaskProgress($taskId: ID!) {
  taskProgress(taskId: $taskId) {
    taskId
    status
    progress
    message
    timestamp
  }
}
```

### Provider Health Updates

```graphql
subscription ProviderHealth {
  providerHealthUpdates {
    providerID
    status {
      status
      latency
      errorMessage
    }
    timestamp
  }
}
```

### Token Streaming

```graphql
subscription TokenStream($requestId: ID!) {
  tokenStream(requestId: $requestId) {
    requestId
    token
    isComplete
    tokenCount
    timestamp
  }
}
```

## Pagination

GraphQL API uses Relay-style cursor pagination:

```graphql
query PaginatedProviders($first: Int, $after: String) {
  providers(first: $first, after: $after) {
    edges {
      node {
        id
        name
      }
      cursor
    }
    pageInfo {
      hasNextPage
      hasPreviousPage
      startCursor
      endCursor
    }
    totalCount
  }
}

# First page
{ "first": 10 }

# Next page
{ "first": 10, "after": "cursor-from-previous-page" }
```

## Error Handling

GraphQL errors are returned in a standard format:

```json
{
  "data": null,
  "errors": [
    {
      "message": "Provider not found",
      "path": ["provider"],
      "extensions": {
        "code": "NOT_FOUND",
        "statusCode": 404
      }
    }
  ]
}
```

## Best Practices

### 1. Query Only Needed Fields

```graphql
# Good - specific fields
query {
  providers {
    edges {
      node {
        id
        name
        score
      }
    }
  }
}

# Avoid - over-fetching
query {
  providers {
    edges {
      node {
        id
        name
        type
        status
        score
        healthStatus { ... }
        capabilities { ... }
        models { ... }
      }
    }
  }
}
```

### 2. Use Variables

```graphql
# Good - parameterized query
query GetProvider($id: ID!) {
  provider(id: $id) { ... }
}

# Avoid - hardcoded values
query {
  provider(id: "openai") { ... }
}
```

### 3. Batch Queries

```graphql
# Batch multiple queries in one request
query Dashboard {
  providers(first: 5) {
    edges { node { id name score } }
  }
  debates(filter: { status: "running" }, first: 3) {
    edges { node { id topic status } }
  }
  verification {
    overallScore
    verifiedProviders
  }
}
```

## SDK Integration

### JavaScript

```javascript
import { GraphQLClient } from 'graphql-request';

const client = new GraphQLClient('http://localhost:8080/v1/graphql', {
  headers: {
    Authorization: 'Bearer your-api-key'
  }
});

const query = gql`
  query ListProviders {
    providers {
      edges {
        node {
          id
          name
          score
        }
      }
    }
  }
`;

const data = await client.request(query);
```

### Python

```python
from gql import gql, Client
from gql.transport.requests import RequestsHTTPTransport

transport = RequestsHTTPTransport(
    url='http://localhost:8080/v1/graphql',
    headers={'Authorization': 'Bearer your-api-key'}
)

client = Client(transport=transport)

query = gql('''
  query ListProviders {
    providers {
      edges {
        node {
          id
          name
          score
        }
      }
    }
  }
''')

result = client.execute(query)
```

## Related Documentation

- [REST API Reference](/docs/api)
- [Architecture - GraphQL](/docs/architecture/graphql-api.md)
- [Authentication Guide](/docs/guides/authentication.md)
