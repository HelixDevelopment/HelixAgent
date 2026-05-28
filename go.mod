module dev.helix.agent

go 1.26

require github.com/gin-gonic/gin v1.12.0

require (
	dev.helix.agent/pkg/api v0.0.0-00010101000000-000000000000
	digital.vasic.agentic v0.0.0-00010101000000-000000000000
	digital.vasic.auth v0.0.0-00010101000000-000000000000
	digital.vasic.background v0.0.0
	digital.vasic.benchmark v0.0.0-00010101000000-000000000000
	digital.vasic.cache v0.0.0-00010101000000-000000000000
	digital.vasic.challenges v0.0.0
	digital.vasic.concurrency v0.0.0-00010101000000-000000000000
	digital.vasic.containers v0.0.0-00010101000000-000000000000
	digital.vasic.database v0.0.0-00010101000000-000000000000
	digital.vasic.debate v0.0.0-00010101000000-000000000000
	digital.vasic.eventbus v0.0.0-00010101000000-000000000000
	digital.vasic.formatters v0.0.0-00010101000000-000000000000
	digital.vasic.helixmemory v0.0.0-00010101000000-000000000000
	digital.vasic.helixqa v0.0.0-00010101000000-000000000000
	digital.vasic.helixspecifier v0.0.0-00010101000000-000000000000
	digital.vasic.llmops v0.0.0-00010101000000-000000000000
	digital.vasic.llmprovider v0.0.0
	digital.vasic.llmsverifier v0.0.0
	digital.vasic.mcp v0.0.0-00010101000000-000000000000
	digital.vasic.memory v0.0.0-00010101000000-000000000000
	digital.vasic.messaging v0.0.0-00010101000000-000000000000
	digital.vasic.models v0.0.0
	digital.vasic.normalize v0.0.0-00010101000000-000000000000
	digital.vasic.optimization v0.0.0-00010101000000-000000000000
	digital.vasic.planning v0.0.0-00010101000000-000000000000
	digital.vasic.plugins v0.0.0-00010101000000-000000000000
	digital.vasic.rag v0.0.0-00010101000000-000000000000
	digital.vasic.redteam v0.0.0-00010101000000-000000000000
	digital.vasic.security v0.0.0-00010101000000-000000000000
	digital.vasic.selfimprove v0.0.0-00010101000000-000000000000
	digital.vasic.storage v0.0.0-00010101000000-000000000000
	digital.vasic.streaming v0.0.0-00010101000000-000000000000
	digital.vasic.toolschema v0.0.0-00010101000000-000000000000
	digital.vasic.vectordb v0.0.0-00010101000000-000000000000
	digital.vasic.visionengine v0.0.0-00010101000000-000000000000
	github.com/ClickHouse/clickhouse-go/v2 v2.41.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/HelixDevelopment/helix_agent/Toolkit v0.0.0-20260209162635-acd6d0755327
	github.com/alecthomas/chroma v0.10.0
	github.com/alicebob/miniredis/v2 v2.36.1
	github.com/andybalholm/brotli v1.2.0
	github.com/docker/docker v28.5.2+incompatible
	github.com/fatih/color v1.19.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-enry/go-enry/v2 v2.9.5
	github.com/go-git/go-git/v5 v5.18.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/graphql-go/graphql v0.8.1
	github.com/jackc/pgx/v5 v5.9.2
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.12.3
	github.com/minio/minio-go/v7 v7.0.100
	github.com/neo4j/neo4j-go-driver/v5 v5.28.4
	github.com/playwright-community/playwright-go v0.5700.1
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/quic-go/quic-go v0.59.0
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/redis/go-redis/v9 v9.18.0
	github.com/segmentio/kafka-go v0.4.49
	github.com/sirupsen/logrus v1.9.4
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
	github.com/sourcegraph/jsonrpc2 v0.2.1
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.43.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.40.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.40.0
	go.opentelemetry.io/otel/metric v1.43.0
	go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/trace v1.43.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.51.0
	golang.org/x/sync v0.20.0
	golang.org/x/sys v0.44.0
	golang.org/x/text v0.37.0
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.44.2
)

require (
	dario.cat/mergo v1.0.2 // indirect
	digital.vasic.docprocessor v0.0.0-00010101000000-000000000000 // indirect
	digital.vasic.llmorchestrator v0.0.0-00010101000000-000000000000 // indirect
	github.com/ClickHouse/ch-go v0.69.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/deckarep/golang-set/v2 v2.8.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.8.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.37 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/paulmach/orb v0.12.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/net v0.53.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gotest.tools/v3 v3.5.2 // indirect
	modernc.org/libc v1.67.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace dev.helix.agent/pkg/api => ./pkg/api

replace digital.vasic.containers => ../containers

replace digital.vasic.challenges => ../challenges

replace digital.vasic.agentic => ../dependencies/vasic-digital/agentic

replace digital.vasic.llmops => ../dependencies/vasic-digital/LLMOps

replace digital.vasic.selfimprove => ../dependencies/vasic-digital/self_improve

replace digital.vasic.planning => ../dependencies/vasic-digital/planning

replace digital.vasic.benchmark => ../dependencies/vasic-digital/benchmark

replace digital.vasic.llmsverifier => ../dependencies/HelixDevelopment/llms_verifier/llm-verifier

replace digital.vasic.auth => ../dependencies/vasic-digital/auth

replace digital.vasic.cache => ../dependencies/vasic-digital/cache

replace digital.vasic.concurrency => ../dependencies/vasic-digital/concurrency

replace digital.vasic.database => ../dependencies/vasic-digital/database

replace digital.vasic.embeddings => ../dependencies/vasic-digital/embeddings

replace digital.vasic.eventbus => ../dependencies/vasic-digital/event_bus

replace digital.vasic.helixmemory => ../dependencies/HelixDevelopment/helix_memory

replace digital.vasic.helixspecifier => ../dependencies/HelixDevelopment/helix_specifier

replace digital.vasic.formatters => ../dependencies/vasic-digital/formatters

replace digital.vasic.mcp => ../dependencies/vasic-digital/MCP_Module

replace digital.vasic.memory => ../dependencies/vasic-digital/memory

replace digital.vasic.messaging => ../dependencies/vasic-digital/messaging

replace digital.vasic.observability => ../dependencies/vasic-digital/observability

replace digital.vasic.optimization => ../dependencies/vasic-digital/optimization

replace digital.vasic.plugins => ../dependencies/vasic-digital/plugins

replace digital.vasic.rag => ../dependencies/vasic-digital/RAG

replace digital.vasic.security => ../security

replace digital.vasic.storage => ../dependencies/vasic-digital/storage

replace digital.vasic.streaming => ../dependencies/vasic-digital/streaming

replace digital.vasic.vectordb => ../dependencies/vasic-digital/vector_db

replace digital.vasic.toolschema => ../dependencies/vasic-digital/tool_schema

replace digital.vasic.skillregistry => ../dependencies/vasic-digital/skill_registry

replace digital.vasic.conversation => ../dependencies/vasic-digital/conversation

replace digital.vasic.models => ../dependencies/vasic-digital/models

replace digital.vasic.background => ../dependencies/vasic-digital/background_tasks

replace digital.vasic.llmprovider => ../dependencies/HelixDevelopment/llm_provider

replace digital.vasic.debate => ../dependencies/HelixDevelopment/debate_orchestrator

replace digital.vasic.helixqa => ../helix_qa

replace digital.vasic.docprocessor => ../dependencies/HelixDevelopment/doc_processor

replace digital.vasic.llmorchestrator => ../dependencies/HelixDevelopment/llm_orchestrator

replace digital.vasic.visionengine => ../dependencies/HelixDevelopment/vision_engine

replace digital.vasic.normalize => ../dependencies/vasic-digital/normalize

replace digital.vasic.redteam => ../dependencies/vasic-digital/red_team

replace github.com/HelixDevelopment/helix_agent/Toolkit => ./Toolkit
