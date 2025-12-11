#!/bin/bash

# SiliconFlow Setup Script for OpenCode & Crush
echo "🔧 Setting up SiliconFlow for OpenCode and Crush..."

# 1. Set your API key securely
read -sp "Enter your SiliconFlow API Key: " API_KEY
echo ""
echo "API Key received (showing first/last 4 chars): ${API_KEY:0:4}...${API_KEY: -4}"

# 2. Create backup of original configs
echo "📦 Creating backups..."
BACKUP_DIR="$HOME/.config/crush_backup_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup Crush config if exists
if [ -f "$HOME/.config/crush/crush.json" ]; then
    cp "$HOME/.config/crush/crush.json" "$BACKUP_DIR/crush.json.backup"
    echo "  ✓ Crush config backed up"
fi

# Backup OpenCode config if exists
if [ -f "$HOME/.config/opencode/config.json" ]; then
    cp "$HOME/.config/opencode/config.json" "$BACKUP_DIR/config.json.backup"
    echo "  ✓ OpenCode config backed up"
fi

# 3. Create config directories
echo "📁 Creating config directories..."
mkdir -p "$HOME/.config/crush"
mkdir -p "$HOME/.config/opencode"

# 4. Generate Crush config with API key
echo "⚙️ Generating Crush configuration..."
cat > "$HOME/.config/crush/crush.json" << EOF
{
    "\$schema": "https://charm.land/crush.json",
    "providers": {
        "siliconflow-primary": {
            "name": "SiliconFlow (Primary)",
            "type": "openai-compat",
            "api_key": "$API_KEY",
            "base_url": "https://api.siliconflow.com/v1",
            "models": [
                {
                    "id": "Qwen/Qwen2.5-72B-Instruct",
                    "name": "Qwen2.5-72B-Instruct (Main)",
                    "context_window": 131072,
                    "default_max_tokens": 32000,
                    "can_reason": true,
                    "supports_attachments": true
                },
                {
                    "id": "Qwen/Qwen2.5-Coder-32B-Instruct",
                    "name": "Qwen2.5-Coder-32B (Code Specialist)",
                    "context_window": 131072,
                    "default_max_tokens": 32000,
                    "can_reason": true,
                    "supports_attachments": true
                }
            ]
        }
    },
    "defaultProvider": "siliconflow-primary",
    "defaultModel": "Qwen/Qwen2.5-72B-Instruct"
}
EOF
echo "  ✓ Crush config created at ~/.config/crush/crush.json"

# 5. Generate OpenCode config with API key
echo "⚙️ Generating OpenCode configuration..."
cat > "$HOME/.config/opencode/config.json" << EOF
{
  "\$schema": "https://opencode.ai/config.json",
  "theme": "dark",
  "model": "Qwen/Qwen2.5-72B-Instruct",
  "small_model": "Qwen/Qwen2.5-7B-Instruct",
  "provider": {
    "siliconflow": {
      "name": "SiliconFlow Provider",
      "npm": "@ai-sdk/openai-compatible",
      "models": {
        "Qwen/Qwen2.5-72B-Instruct": {
          "name": "Qwen2.5 72B (Main)"
        },
        "Qwen/Qwen2.5-Coder-32B-Instruct": {
          "name": "Qwen2.5 Coder 32B"
        }
      },
      "options": {
        "apiKey": "$API_KEY",
        "baseURL": "https://api.siliconflow.com/v1"
      }
    }
  }
}
EOF
echo "  ✓ OpenCode config created at ~/.config/opencode/config.json"

# 6. Test the connection
echo "🧪 Testing SiliconFlow connection..."
TEST_RESPONSE=$(curl -s -X POST "https://api.siliconflow.com/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [{"role": "user", "content": "Say OK if working"}],
    "max_tokens": 10
  }')

if echo "$TEST_RESPONSE" | grep -q "OK"; then
    echo "  ✅ Connection successful!"
else
    echo "  ⚠️  Connection test inconclusive. Please verify manually."
    echo "  Response preview: ${TEST_RESPONSE:0:100}..."
fi

# 7. Set secure permissions
echo "🔐 Setting secure permissions..."
chmod 600 "$HOME/.config/crush/crush.json"
chmod 600 "$HOME/.config/opencode/config.json"

echo ""
echo "========================================="
echo "✅ Setup Complete!"
echo "========================================="
echo "📋 Summary:"
echo "   • Backups created in: $BACKUP_DIR"
echo "   • Crush config: ~/.config/crush/crush.json"
echo "   • OpenCode config: ~/.config/opencode/config.json"
echo ""
echo "🚀 Next steps:"
echo "   1. Restart Crush and OpenCode"
echo "   2. Test with a simple prompt"
echo "   3. Adjust models in configs as needed"
echo ""
echo "🔧 Troubleshooting:"
echo "   • Check API key at: https://cloud.siliconflow.cn/account/ak"
echo "   • Verify key has sufficient credits"
echo "   • Ensure network can reach api.siliconflow.com"
echo "========================================="

