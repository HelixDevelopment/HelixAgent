# Deployment Configuration Reference

This document provides the environment variable reference for deploying HelixAgent.

## Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `7061` | HTTP server port |
| `GIN_MODE` | `release` | Gin framework mode (`debug`, `release`, `test`) |
| `JWT_SECRET` | (required) | Secret for JWT token signing |

## Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `helixagent` | Database user |
| `DB_PASSWORD` | (required) | Database password |
| `DB_NAME` | `helixagent_db` | Database name |

## Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | (required) | Redis password |

## LLM Providers

Each provider uses `<PROVIDER>_API_KEY` for authentication. OAuth-based providers (Claude, Qwen) also support `<PROVIDER>_USE_OAUTH_CREDENTIALS=true`.

See `.env.example` in the project root for the complete list of provider-specific variables.

## Service Overrides

Individual services can be overridden with `SVC_<SERVICE>_<FIELD>` variables:

```bash
SVC_POSTGRESQL_HOST=remote-db.example.com
SVC_REDIS_REMOTE=true
```

## Container Orchestration

Container behavior is controlled via `containers/.env`:

| Variable | Description |
|----------|-------------|
| `CONTAINERS_REMOTE_ENABLED` | `true` to deploy all containers to remote hosts |
| `CONTAINERS_REMOTE_HOST_*` | Remote host connection details |

## Configuration Files

- `configs/development.yaml` - Development settings
- `configs/production.yaml` - Production settings
- `.env` - Environment variables (do not commit)

## Related Documentation

- [General Deployment Guide](./general-deployment-guide.md)
- [Services Configuration](../user/SERVICES_CONFIGURATION.md)
- [Remote Deployment](./REMOTE_DEPLOYMENT.md)
