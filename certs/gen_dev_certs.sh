#!/usr/bin/env bash
# certs/gen_dev_certs.sh — regenerate the HelixAgent DEV-ONLY self-signed TLS
# keypair.
#
# NOTE: this script lives under certs/ (not scripts/) deliberately — it is
# scoped to this directory's own regeneration mechanism and does not touch
# or depend on anything under scripts/.
#
# Why this exists (CONST-053 / §11.4.30 + §11.4.77 regeneration mandate):
#   certs/{cert.pem,key.pem} used to be committed to git. They are private
#   key material and MUST NOT be versioned. Once excluded via .gitignore,
#   the codebase must still be able to reproduce an equivalent artifact
#   out of the box — that is what this script does.
#
#   Note: HelixAgent's own production HTTP/3 listener (cmd/helixagent/main.go)
#   does NOT read these files — it auto-generates an in-memory self-signed
#   cert at startup (TLSCertFile/TLSKeyFile left empty). These on-disk
#   certs/{cert.pem,key.pem} are a standalone dev/test TLS keypair (e.g. for
#   local QUIC/mTLS experiments or docker-compose bind mounts that expect a
#   file on disk) and are kept working the same way regardless.
#
# What it produces:
#   certs/cert.pem — self-signed X.509 certificate, CN=localhost
#   certs/key.pem  — matching RSA-4096 private key
#
# Usage:
#   ./certs/gen_dev_certs.sh          # generate only if missing (idempotent)
#   ./certs/gen_dev_certs.sh --force  # always regenerate
#
# These are DEV-ONLY self-signed certificates. Never use them in production;
# never commit the regenerated files (see .gitignore).
set -euo pipefail

CERT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_FILE="${CERT_DIR}/cert.pem"
KEY_FILE="${CERT_DIR}/key.pem"
CN="localhost"
DAYS="${HELIX_AGENT_DEV_CERT_DAYS:-365}"
FORCE=0

for arg in "$@"; do
  case "$arg" in
    --force|-f) FORCE=1 ;;
    -h|--help)
      sed -n '2,29p' "${BASH_SOURCE[0]}"
      exit 0
      ;;
    *)
      echo "gen_dev_certs.sh: unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

if ! command -v openssl >/dev/null 2>&1; then
  echo "gen_dev_certs.sh: openssl not found on PATH — cannot generate dev certs" >&2
  exit 1
fi

mkdir -p "${CERT_DIR}"

if [ "${FORCE}" -eq 0 ] && [ -f "${CERT_FILE}" ] && [ -f "${KEY_FILE}" ]; then
  echo "gen_dev_certs.sh: ${CERT_FILE} and ${KEY_FILE} already present — skipping (use --force to regenerate)"
  exit 0
fi

echo "gen_dev_certs.sh: generating self-signed dev TLS keypair (CN=${CN}, ${DAYS} days) ..."
openssl req -x509 -newkey rsa:4096 \
  -keyout "${KEY_FILE}" -out "${CERT_FILE}" \
  -days "${DAYS}" -nodes \
  -subj "/CN=${CN}" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" 2>/dev/null

chmod 600 "${KEY_FILE}"
chmod 644 "${CERT_FILE}"

echo "gen_dev_certs.sh: done"
echo "  cert: ${CERT_FILE}"
echo "  key:  ${KEY_FILE}"
openssl x509 -in "${CERT_FILE}" -noout -subject -dates
