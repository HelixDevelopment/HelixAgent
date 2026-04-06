# Module 16: HTTP/3 (QUIC) and Brotli Compression

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 16: HTTP/3 (QUIC) and Brotli Compression
- Duration: 60 minutes
- Modern Transport for AI Infrastructure

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand HTTP/3 (QUIC) advantages over HTTP/2
- Configure HelixAgent for QUIC transport
- Implement Brotli compression with gzip fallback
- Verify transport and compression in production

---

## Slide 3: Why HTTP/3?

**HTTP/2 Limitations vs HTTP/3 Advantages:**

| Feature | HTTP/2 | HTTP/3 (QUIC) |
|---------|--------|----------------|
| Transport | TCP | UDP (QUIC) |
| Head-of-line blocking | Yes (TCP level) | No (per-stream) |
| Connection setup | TCP + TLS (2-3 RTT) | 0-1 RTT |
| Connection migration | No | Yes (IP change OK) |
| Multiplexing | Streams share TCP | Independent streams |

*For LLM streaming responses, eliminating head-of-line blocking is critical*

---

## Slide 4: QUIC Transport Architecture

**HelixAgent Transport Stack:**

```
Application Layer
    |
    v
+------------------+
| HTTP/3 (primary) |  <-- quic-go/quic-go
+--------+---------+
         |
         | Fallback if QUIC unavailable
         v
+------------------+
| HTTP/2 (fallback)|
+--------+---------+
         |
         v
+------------------+
| TLS 1.3          |  <-- Required for QUIC
+------------------+
```

---

## Slide 5: QUIC Configuration

**Server-Side QUIC:**

- HelixAgent listens on both TCP (HTTP/2) and UDP (QUIC) simultaneously
- TLS 1.3 is mandatory for QUIC connections
- Certificate configuration shared between HTTP/2 and HTTP/3

**Client-Side QUIC:**

- All outbound HTTP calls prefer HTTP/3
- Automatic fallback to HTTP/2 when QUIC is unavailable
- Connection pooling via `internal/adapters/http/` client pool adapter

---

## Slide 6: Brotli Compression

**Why Brotli over gzip?**

| Metric | gzip | Brotli |
|--------|------|--------|
| Compression ratio | Good | 15-25% better |
| Decompression speed | Fast | Comparable |
| Compression speed | Fast | Slower (but configurable) |
| Browser support | Universal | Modern browsers |
| Library | stdlib | `andybalholm/brotli` |

*For API responses (JSON), Brotli typically achieves 20%+ smaller payloads than gzip*

---

## Slide 7: Compression Priority

**HelixAgent Compression Chain:**

```
Client Request
  Accept-Encoding: br, gzip, deflate
         |
         v
+------------------+
| Check: br?       |--Yes--> Brotli compress
+--------+---------+
         | No
         v
+------------------+
| Check: gzip?     |--Yes--> gzip compress
+--------+---------+
         | No
         v
    No compression
```

**Priority: Brotli (primary) then gzip (fallback)**

---

## Slide 8: Compression Middleware

**How compression is applied:**

- Response middleware compresses all responses above a size threshold
- Content-Encoding header set automatically
- Streaming responses use chunked Brotli encoding
- Static assets pre-compressed where possible
- Compression level tunable for speed vs ratio tradeoff

---

## Slide 9: Verifying QUIC and Brotli

**Testing Transport and Compression:**

```bash
# Check if QUIC is active (look for Alt-Svc header)
curl -v http://localhost:7061/health 2>&1 | grep -i "alt-svc"

# Request Brotli compression
curl -H "Accept-Encoding: br" \
  http://localhost:7061/v1/models \
  --compressed -o /dev/null -w "Size: %{size_download}\n"

# Compare with gzip
curl -H "Accept-Encoding: gzip" \
  http://localhost:7061/v1/models \
  --compressed -o /dev/null -w "Size: %{size_download}\n"

# Run the Brotli compression challenge
./challenges/scripts/brotli_compression_challenge.sh
```

---

## Slide 10: Hands-On Lab

**Lab Exercise 16.1: HTTP/3 and Brotli Verification**

Tasks:
1. Verify QUIC transport is active via connection metadata
2. Test Brotli compression on API responses
3. Compare compressed sizes: Brotli vs gzip
4. Measure latency with and without QUIC
5. Verify fallback behavior when QUIC is unavailable

Time: 30 minutes

---

## Slide 11: Module Summary

**Key Takeaways:**

- HTTP/3 (QUIC) eliminates head-of-line blocking for LLM streaming
- 0-1 RTT connection setup reduces latency
- Brotli compression achieves 15-25% smaller payloads than gzip
- Compression priority: Brotli (primary), gzip (fallback)
- TLS 1.3 is required for QUIC transport
- All HTTP clients and servers prefer HTTP/3 by default

**Next: Module 17 - Remote Container Distribution**

---

## Speaker Notes

### Slide 3 Notes
The key selling point for HTTP/3 in AI infrastructure is streaming. When multiple
LLM responses stream simultaneously, TCP head-of-line blocking causes one slow
stream to delay all others. QUIC eliminates this entirely.

### Slide 6 Notes
Brotli was developed by Google specifically for web content. JSON API responses
compress exceptionally well with Brotli due to repetitive structure.

### Slide 9 Notes
Demo the size comparison live. Typical JSON API response savings: gzip ~70% reduction,
Brotli ~75-80% reduction. The difference adds up at scale.
