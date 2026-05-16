# Snyk Security Scanner Container for HelixAgent
FROM snyk/snyk-cli:latest

RUN apt-get update && apt-get install -y curl jq git \
    && rm -rf /var/lib/apt/lists/*

RUN curl -L https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH=$PATH:/usr/local/go/bin
ENV GOPATH=/go
ENV GOBIN=$GOPATH/bin
ENV PATH=$PATH:$GOBIN

WORKDIR /app
COPY . .

RUN mkdir -p /scripts /reports

RUN cat > /scripts/scan-dependencies.sh << 'SCRIPT_EOF'
#!/bin/bash
set -e
echo "=== HelixAgent Snyk Dependency Scan ==="
mkdir -p /reports
cd /app
snyk test --json --severity-threshold=low > /reports/snyk-deps.json 2>&1 || true
echo "Dependency scan complete. Report: /reports/snyk-deps.json"
jq -r '.vulnerabilities | length' /reports/snyk-deps.json 2>/dev/null && echo " vulnerabilities found" || true
SCRIPT_EOF

RUN cat > /scripts/scan-code.sh << 'SCRIPT_EOF'
#!/bin/bash
set -e
echo "=== HelixAgent Snyk Code Scan ==="
mkdir -p /reports
cd /app
snyk code test --json --severity-threshold=low > /reports/snyk-code.json 2>&1 || true
echo "Code scan complete. Report: /reports/snyk-code.json"
SCRIPT_EOF

RUN cat > /scripts/scan-iac.sh << 'SCRIPT_EOF'
#!/bin/bash
set -e
echo "=== HelixAgent Snyk IaC Scan ==="
mkdir -p /reports
cd /app
snyk iac test --json docker/ containers/ > /reports/snyk-iac.json 2>&1 || true
echo "IaC scan complete. Report: /reports/snyk-iac.json"
SCRIPT_EOF

RUN cat > /scripts/scan-all.sh << 'SCRIPT_EOF'
#!/bin/bash
set -e
echo "=== HelixAgent Full Snyk Security Scan ==="
mkdir -p /reports
/scripts/scan-dependencies.sh
/scripts/scan-code.sh
/scripts/scan-iac.sh
echo "=== All scans complete ==="
echo "Reports available in /reports/"
ls -la /reports/
SCRIPT_EOF

RUN chmod +x /scripts/*.sh
CMD ["/scripts/scan-all.sh"]
