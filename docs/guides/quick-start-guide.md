# HelixAgent Quick Start Guide

## 🚀 Get Started in 5 Minutes

This guide will help you get HelixAgent running and making your first AI-powered requests in under 5 minutes.

### Prerequisites

- **Docker & Docker Compose** (recommended for quick setup)
- **curl** or **Postman** for API testing
- **API keys** for at least one LLM provider (Claude, DeepSeek, or Gemini)

---

## 📦 Option 1: Docker (Fastest & Easiest)

### Step 1: Clone and Setup
```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Create environment file
cp .env.example .env
```

### Step 2: Configure API Keys
Edit `.env` file and add your API keys:
```bash
# Add at least one of these:
CLAUDE_API_KEY=sk-ant-api03-your-claude-key-here
DEEPSEEK_API_KEY=sk-your-deepseek-key-here
GEMINI_API_KEY=your-gemini-api-key-here

# Optional: Add these for more providers
QWEN_API_KEY=your-qwen-key-here
ZAI_API_KEY=your-zai-key-here
```

### Step 3: Start HelixAgent
```bash
# Start all services (AI + Monitoring)
make docker-full

# Or just the AI services
make docker-ai
```

### Step 4: Verify Installation
```bash
# Check that services are healthy
curl http://localhost:7061/v1/health

# List available providers (should show your configured providers)
curl http://localhost:7061/v1/providers

# Check provider health with circuit breaker status
curl http://localhost:7061/v1/providers/claude/health

# Expected response for /v1/health:
# {"status":"healthy","providers":{"claude":{"status":"healthy"},...}}
```

---

## 💻 Option 2: Local Development (Go)

### Step 1: Install Dependencies
```bash
# Install Go dependencies
make install-deps

# Setup development environment
make setup-dev
```

### Step 2: Configure Environment
```bash
# Export API keys
export CLAUDE_API_KEY="sk-ant-api03-your-claude-key-here"
export DEEPSEEK_API_KEY="sk-your-deepseek-key-here"
export GEMINI_API_KEY="your-gemini-api-key-here"
```

### Step 3: Run HelixAgent
```bash
# Run in development mode
make run-dev

# Or run directly
go run ./cmd/helixagent/main.go
```

---

## 🎯 Your First API Request

### Test 1: Simple Completion
```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, what is HelixAgent?",
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 100
  }'
```

### Test 2: Chat Completion
```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Write a simple Go function to reverse a string"
      }
    ]
  }'
```

### Test 3: Ensemble Magic (Multiple Providers)
```bash
curl -X POST http://localhost:7061/v1/ensemble/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain quantum computing in simple terms",
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 2
    }
  }'
```

### Test 4: Streaming Responses
```bash
curl -X POST http://localhost:7061/v1/completions/stream \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a short story about a robot learning emotions",
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 200,
    "stream": true
  }'
```

---

## 🔍 Verify Everything Works

### Check Available Providers
```bash
curl http://localhost:7061/v1/providers
```

### Check Provider Health
```bash
curl http://localhost:7061/v1/providers/claude/health
```

### View Available Models
```bash
curl http://localhost:7061/v1/models
```

### Monitor System Metrics
```bash
curl http://localhost:7061/metrics
```

---

## 🛠️ Next Steps

### 1. **Add Authentication**
```bash
# Get an API key
curl -X POST http://localhost:7061/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword123"
  }'

# Login to get JWT token
curl -X POST http://localhost:7061/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword123"
  }'
```

### 2. **Use Your API Key**
```bash
# Use the JWT token in Authorization header
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### 3. **Explore Monitoring Dashboard**
Open in browser: http://localhost:3000
- Username: `admin`
- Password: `admin`

---

## 🚨 Common Issues & Solutions

### Issue: "Provider not available"
**Solution:** Check your API keys in `.env` file and restart services.

### Issue: "Port 8080 already in use"
**Solution:** Change port in `.env`:
```bash
PORT=8081
```

### Issue: "Database connection failed"
**Solution:** Ensure PostgreSQL is running:
```bash
docker-compose ps
docker-compose logs postgres
```

### Issue: "Rate limit exceeded"
**Solution:** Wait a few minutes or check your provider's rate limits.

---

## 📚 Learn More

- **API Documentation**: See `/docs/api-documentation.md`
- **API Examples**: See `/docs/api-reference-examples.md`
- **Multi-Provider Setup**: See `/docs/MULTI_PROVIDER_SETUP.md`
- **Architecture**: See `/docs/architecture.md`

---

## 🎉 Congratulations!

You've successfully:
1. ✅ Set up HelixAgent
2. ✅ Made your first API requests
3. ✅ Tested ensemble intelligence
4. ✅ Verified system health

**Ready for production?** Check out `/docs/production-deployment.md` for advanced configuration and scaling options.

---

## 🆘 Need Help?

1. **Check logs**: `docker-compose logs helixagent`
2. **Verify configuration**: `cat .env`
3. **Test connectivity**: `curl http://localhost:7061/health`
4. **Review documentation**: All docs are in `/docs/` directory

---

**Next:** Learn about [Configuration Guide](./configuration-guide.md) →