# Deployment Troubleshooting

Common issues and solutions for HelixAgent deployment.

## Startup Failures

### Provider Verification Fails

**Symptom:** Server does not start; logs show provider verification errors.

**Fix:** Ensure at least one LLM provider API key is configured in `.env`. Check the startup verification endpoint at `/v1/startup/verification` for details.

### Database Connection Refused

**Symptom:** `connection refused` errors for PostgreSQL.

**Fix:** Verify `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, and `DB_NAME` are correct. Ensure PostgreSQL is running and accepting connections. Run `make test-infra-start` for local development.

### Redis Connection Timeout

**Symptom:** Redis health check fails during boot.

**Fix:** Confirm `REDIS_HOST` and `REDIS_PORT` point to a running Redis instance. For test infrastructure, the default port is `16379` (not `6379`).

## Container Issues

### Containers Not Starting

**Symptom:** Services fail health checks after boot.

**Fix:** HelixAgent manages containers automatically. Check `containers/.env` for correct configuration. Verify Docker or Podman is available with `docker info` or `podman info`.

### Remote Distribution Fails

**Symptom:** SSH errors when deploying to remote hosts.

**Fix:** Ensure SSH keys are configured and `CONTAINERS_REMOTE_HOST_*` variables in `containers/.env` are correct. All connections must use key-based authentication.

## Performance Issues

### High Memory Usage

**Symptom:** Memory grows unbounded.

**Fix:** Check for goroutine leaks with `/debug/pprof/goroutine`. Verify circuit breakers are configured for all external dependencies.

### Slow Provider Responses

**Symptom:** API latency exceeds expectations.

**Fix:** Enable the debate performance optimizer. Check provider health at `/v1/monitoring/status`. Consider enabling response caching.

## Related Documentation

- [Deployment Guide](./DEPLOYMENT_GUIDE.md)
- [Configuration Reference](./configuration.md)
- [Monitoring](../monitoring/README.md)
