# certs/ — dev-only TLS keypair

This directory holds a **development-only** self-signed TLS certificate and
private key:

- `cert.pem` — self-signed X.509 certificate, `CN=localhost`
- `key.pem`  — matching private key

## Note on what actually consumes these files

HelixAgent's own production HTTP/3 listener (`cmd/helixagent/main.go`) does
**not** read these files at all — it auto-generates an in-memory self-signed
certificate at startup (`TLSCertFile`/`TLSKeyFile` are left empty). This
on-disk keypair is a standalone dev/test TLS artifact (e.g. for local
QUIC/mTLS experiments, or any tooling/compose setup that expects a cert file
on disk at `certs/cert.pem` / `certs/key.pem`) — it is not wired into a
Makefile target the way HelixLLM's is.

## These files are NOT committed

Per CONST-053 / §11.4.30 (no private keys/certs versioned) and the
project-wide anti-bluff mandate, `*.pem` / `*.key` are git-ignored. They
used to be committed by mistake and have been untracked — treat that old
pair as burned and never reuse it for anything beyond local dev.

## Regenerating

```bash
./certs/gen_dev_certs.sh
```

Idempotent — skips generation if both files already exist; pass `--force`
to regenerate unconditionally. Produces a fresh self-signed `CN=localhost`
RSA-4096 keypair (with `subjectAltName=DNS:localhost,IP:127.0.0.1`) valid
for 1 year. **Dev-only** — never use these certificates in production, and
never commit the regenerated files.
