# HelixAgent API Key and OpenCode Configuration Guide

This guide explains how to generate and manage HelixAgent API keys and OpenCode configurations using the HelixAgent binary.

## Overview

HelixAgent provides built-in CLI commands for:
- Generating secure API keys
- Creating OpenCode configuration files
- Managing API keys in environment files

All configuration generation is done through the main HelixAgent binary - no separate scripts required.

## API Key Generation

### Generate a New API Key

```bash
./bin/helixagent -generate-api-key
```

This outputs a cryptographically secure API key in the format:
```
sk-{64_hex_characters}
```

Example output:
```
sk-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

### Generate and Save to .env File

```bash
./bin/helixagent -generate-api-key -api-key-env-file .env
```

This will:
1. Generate a new API key
2. Add/update `HELIXAGENT_API_KEY` in the specified `.env` file
3. Output the generated key to stdout

If the `.env` file exists, it preserves existing content and only updates the `HELIXAGENT_API_KEY` entry.

## OpenCode Configuration Generation

### Generate OpenCode Configuration

```bash
./bin/helixagent -generate-opencode-config
```

This generates a valid OpenCode configuration JSON to stdout. The configuration:
- Uses `HELIXAGENT_API_KEY` from environment (or generates a new one if not set)
- Follows the official OpenCode schema (`https://opencode.ai/config.json`)
- Includes HelixAgent as the provider with the AI Debate model

### Generate and Save to File

```bash
./bin/helixagent -generate-opencode-config -opencode-output opencode.json
```

### Generate with API Key to .env

```bash
./bin/helixagent -generate-opencode-config -opencode-output opencode.json -api-key-env-file .env
```

This command:
1. Checks for `HELIXAGENT_API_KEY` in environment
2. If not found, generates a new key and saves to `.env`
3. Generates OpenCode configuration with the actual API key value
4. Saves configuration to the specified output file

## Generated OpenCode Configuration Structure

The generated configuration follows the LLMsVerifier-validated schema:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "helixagent": {
      "name": "HelixAgent AI Debate Ensemble",
      "options": {
        "apiKey": "sk-...",
        "baseURL": "http://localhost:7061/v1",
        "timeout": 600000
      }
    }
  },
  "agent": {
    "model": "helixagent/helixagent-debate"
  }
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIXAGENT_API_KEY` | API key for authentication | Generated if not set |
| `HELIXAGENT_HOST` | HelixAgent server host | `localhost` |
| `PORT` | HelixAgent server port | `8080` |

## Using with OpenCode

1. Generate the configuration:
   ```bash
   ./bin/helixagent -generate-opencode-config -opencode-output ~/.config/opencode/config.json -api-key-env-file .env
   ```

2. Start HelixAgent:
   ```bash
   ./bin/helixagent
   ```

3. Use with OpenCode:
   ```bash
   opencode
   ```

## Challenge Integration

The main challenge script automatically:
1. Generates an API key if `HELIXAGENT_API_KEY` is not set
2. Saves the key to the project `.env` file
3. Generates OpenCode configuration using the binary
4. Validates the configuration against LLMsVerifier rules

## API Key Format

HelixAgent API keys follow this format:
- Prefix: `sk-`
- Body: 64 hexadecimal characters (256 bits of entropy)
- Example: `sk-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`

## Security Best Practices

1. **Never commit API keys to version control**
   - Add `.env` to `.gitignore`
   - Use `.env.example` with placeholder values

2. **Rotate keys regularly**
   - Generate new keys periodically
   - Update `.env` files after rotation

3. **Use environment-specific keys**
   - Different keys for development, staging, production
   - Store production keys in secure secret management systems

4. **Secure .env file permissions**
   ```bash
   chmod 600 .env
   ```

## CLI Reference

| Flag | Description |
|------|-------------|
| `-generate-api-key` | Generate a new API key and output to stdout |
| `-generate-opencode-config` | Generate OpenCode configuration JSON |
| `-opencode-output <path>` | Output path for OpenCode config (default: stdout) |
| `-api-key-env-file <path>` | Path to .env file for writing the API key |
| `-help` | Show help message with all options |

## Troubleshooting

### API Key Not Persisting

Ensure the `.env` file path is correct and writable:
```bash
./bin/helixagent -generate-api-key -api-key-env-file /full/path/to/.env
```

### OpenCode Config Validation Fails

The binary generates configurations that comply with LLMsVerifier's validation rules. If validation fails:
1. Check that `HELIXAGENT_API_KEY` is set correctly
2. Verify the configuration has required fields
3. Ensure JSON is properly formatted

### HelixAgent Not Using Environment API Key

The service reads `HELIXAGENT_API_KEY` at startup. Ensure:
1. The `.env` file is in the project root
2. The service is restarted after updating the `.env`
3. The key format is correct (`sk-` prefix)
