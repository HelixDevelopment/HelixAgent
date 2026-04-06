# Lab 19: HTTP/3 (QUIC) and Brotli Compression

## Objective
Verify HTTP/3 transport and Brotli compression are active and measure their impact.

## Prerequisites
- HelixAgent built and running (`make build && ./bin/helixagent`)
- curl with HTTP/3 support (optional, for direct QUIC testing)

## Exercise 1: Check Brotli Compression

```bash
# Request with Brotli encoding
curl -s -H "Accept-Encoding: br" \
  http://localhost:7061/v1/models \
  -o /tmp/response-br.bin -D /tmp/headers-br.txt

# Check Content-Encoding header
grep -i "content-encoding" /tmp/headers-br.txt
```

**Expected:** `Content-Encoding: br` present in response headers.

## Exercise 2: Compare Compression Sizes

```bash
# Brotli compressed size
curl -s -H "Accept-Encoding: br" \
  http://localhost:7061/v1/models \
  -o /dev/null -w "Brotli size: %{size_download} bytes\n"

# gzip compressed size
curl -s -H "Accept-Encoding: gzip" \
  http://localhost:7061/v1/models \
  -o /dev/null -w "gzip size: %{size_download} bytes\n"

# Uncompressed size
curl -s -H "Accept-Encoding: identity" \
  http://localhost:7061/v1/models \
  -o /dev/null -w "Uncompressed size: %{size_download} bytes\n"
```

**Expected:** Brotli < gzip < uncompressed.

## Exercise 3: Verify Alt-Svc Header for QUIC

```bash
# Check for HTTP/3 advertisement
curl -v http://localhost:7061/health 2>&1 | grep -i "alt-svc"
```

**Expected:** Alt-Svc header advertising h3 protocol.

## Exercise 4: Run Brotli Challenge

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  ./challenges/scripts/brotli_compression_challenge.sh
```

**Expected:** All 11 tests pass.

## Assessment Questions
1. Why is Brotli preferred over gzip for JSON API responses?
2. What is the fallback behavior when a client does not support Brotli?
3. Why does HTTP/3 require TLS 1.3?
4. How does QUIC eliminate head-of-line blocking for streaming responses?
