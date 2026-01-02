# SGLang Service

SGLang provides RadixAttention for prefix caching and efficient multi-turn conversations.

## Overview

SGLang is a serving framework that provides:
- **RadixAttention**: Efficient KV cache sharing for prefix caching
- **Multi-turn optimization**: Reuse computation across conversation turns
- **Parallel execution**: Process multiple requests efficiently

## Deployment

SGLang is typically deployed using its official Docker image:

```bash
# Start SGLang server with GPU support
docker run -d --gpus all \
  -p 30000:30000 \
  --name sglang \
  lmsysorg/sglang:latest \
  python -m sglang.launch_server \
  --model-path meta-llama/Llama-2-7b-chat-hf \
  --port 30000

# Or use docker-compose with the optimization profile
docker-compose --profile optimization up -d sglang
```

## Configuration

Environment variables:
- `SGLANG_MODEL`: Model to serve (default: uses SuperAgent's configured model)
- `SGLANG_PORT`: Server port (default: 30000)
- `SGLANG_TOKENIZER`: Tokenizer path (optional)

## API Endpoints

The SGLang server exposes OpenAI-compatible endpoints:

- `POST /v1/chat/completions` - Chat completions
- `POST /v1/completions` - Text completions
- `GET /health` - Health check

## Integration with SuperAgent

SuperAgent's SGLang client (`internal/optimization/sglang/`) provides:
- Prefix caching for repeated prompts
- Session management for multi-turn conversations
- Automatic fallback to standard providers if unavailable

## Requirements

- NVIDIA GPU with CUDA support
- Docker with GPU support (nvidia-container-toolkit)
- Sufficient VRAM for the model (7B models require ~14GB)

## Notes

- SGLang runs as a standalone service, not built from source
- For CPU-only environments, use the `fallback_on_unavailable: true` config
- Prefix caching is most effective for:
  - System prompts
  - Few-shot examples
  - Repeated context (like code files)
