# Amazon Q CLI - Usage Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Common Workflows](#common-workflows)
3. [Agent Management](#agent-management)
4. [Knowledge Base](#knowledge-base)
5. [Advanced Features](#advanced-features)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

---

## Getting Started

### First-Time Setup

1. **Install Amazon Q CLI**
   ```bash
   # macOS with Homebrew
   brew install --cask amazon-q
   
   # Or download DMG from AWS
   ```

2. **Authenticate**
   ```bash
   q chat
   # Follow browser OAuth flow with AWS
   ```

3. **Verify Installation**
   ```bash
   q status
   ```

### Basic Navigation

```bash
# Navigate to your project
cd ~/projects/my-app

# Start Amazon Q
q chat

# You'll see the Q prompt waiting for input
```

---

## Common Workflows

### 1. Code Exploration

**Understanding a New Codebase**

```
> What does this project do?

Q will:
1. Read README.md
2. Analyze project structure
3. Explore directory contents
4. Summarize architecture
```

**Finding Specific Code**

```
> Find where user authentication is handled

Q will:
1. Search for auth-related files
2. Read relevant code
3. Explain the flow
4. Show key files
```

**Explaining Complex Code**

```
> Explain the logic in src/utils/cache.rs

Q will:
1. Read the file
2. Break down complex sections
3. Provide context
4. Suggest improvements
```

### 2. Code Modifications

**Making Edits**

```
> Add input validation to the login function

Q will:
1. Find the login function
2. Propose changes
3. Show diff for approval
4. Apply with your confirmation
```

**Refactoring**

```
> Refactor this code to use async/await instead of callbacks

Q will:
1. Identify callback patterns
2. Convert to async/await
3. Handle error cases
4. Test the changes
```

**Creating New Files**

```
> Create a middleware for JWT authentication

Q will:
1. Check existing middleware structure
2. Create auth middleware
3. Implement JWT verification
4. Add to exports
```

### 3. AWS Integration

**Exploring AWS Resources**

```
> List my S3 buckets

Q uses use_aws tool:
- Service: s3
- Operation: list-buckets
```

**Lambda Operations**

```
> Show me my Lambda functions in us-east-1

Q will:
1. Call AWS Lambda API
2. Display function list
3. Show configuration details
```

**CloudFormation Stacks**

```
> Check the status of my CloudFormation stacks

Q will query CloudFormation and show stack status
```

### 4. Testing & Debugging

**Running Tests**

```
> Run the test suite

> Run tests for the auth module only

> Fix the failing test in users.test.ts
```

**Debugging**

```
> Why is this test failing?

> Find the source of this error: [paste error]

> Add logging to trace this issue
```

### 5. Project Setup

**New Project**

```
> Set up a new Rust project with cargo

Q will:
1. Run cargo init
2. Set up basic structure
3. Create initial files
```

**Adding Dependencies**

```
> Add tokio and serde to this project

> Set up AWS SDK for Rust
```

---

## Agent Management

### Creating Custom Agents

**Interactive Generation:**

```
> /agent generate

Q will ask questions and create an agent configuration
```

**Manual Creation:**

Create file at `~/.aws/amazonq/cli-agents/my-agent.json`:

```json
{
  "name": "rust-expert",
  "description": "Expert in Rust development",
  "prompt": "You are a Rust expert. Follow best practices...",
  "tools": ["fs_read", "fs_write", "execute_bash"],
  "allowedTools": ["fs_read"],
  "toolsSettings": {
    "fs_write": {
      "allowedPaths": ["src/**", "Cargo.toml"]
    }
  }
}
```

### Using Agents

```bash
# Start with specific agent
q chat --agent rust-expert

# Set default agent
q settings chat.defaultAgent rust-expert

# List available agents
> /agent list
```

### Agent Best Practices

1. **Scope tools narrowly**: Only include tools the agent needs
2. **Set allowed paths**: Restrict file system access
3. **Use descriptive names**: Clear agent purpose
4. **Version your agents**: Track changes in git

---

## Knowledge Base

### Enabling Knowledge

```bash
# Enable knowledge feature
q settings chat.enableKnowledge true
```

### Adding Knowledge

**Basic Usage:**

```
> /knowledge add -n "project-docs" -p ./docs

Q will index the docs directory for semantic search
```

**With Index Type:**

```
> /knowledge add -n "api-docs" -p ./api-docs --index-type Best

Best = Semantic search (slower, more intelligent)
Fast = BM25 search (faster, keyword-based)
```

**With Patterns:**

```
> /knowledge add -n "rust-code" -p ./src --include "*.rs" --exclude "target/**"
```

### Using Knowledge

```
> How do I authenticate users in this codebase?

Q will search knowledge base and provide answers based on indexed content
```

### Managing Knowledge

```
# Show all entries
> /knowledge show

# Update an entry
> /knowledge update ./docs

# Remove an entry
> /knowledge remove "project-docs"

# Clear all knowledge
> /knowledge clear
```

### Knowledge Best Practices

- **Use descriptive names**: "api-v2-docs" not "docs"
- **Index relevant content only**: Use patterns to filter
- **Choose right index type**: Fast for code, Best for docs
- **Update regularly**: Keep knowledge fresh
- **Organize by topic**: Separate knowledge bases for different domains

---

## Advanced Features

### TODO Lists

**Automatic Creation:**

Q automatically creates TODO lists for complex tasks:

```
> Refactor the authentication module

Q creates a task list:
🛠️  Using tool: todo_list

● Tasks:
[ ] Analyze current auth implementation
[ ] Design new auth structure
[ ] Implement new auth middleware
[ ] Update tests
```

**Managing TODOs:**

```
# View all TODO lists
> /todos view

# Resume a TODO list
> /todos resume

# Clear completed lists
> /clear-finished

# Delete specific lists
> /todos delete
```

### Hooks

**Audit Logging:**

Create `.amazonq/cli-agents/hooks/audit.sh`:

```bash
#!/bin/bash
# Log all tool usage
read event
echo "$(date) - $event" >> ~/.amazonq/audit.log
```

Add to agent config:

```json
{
  "hooks": {
    "preToolUse": [
      {
        "matcher": "*",
        "command": "~/.amazonq/cli-agents/hooks/audit.sh"
      }
    ]
  }
}
```

**Auto-Format on Save:**

```json
{
  "hooks": {
    "postToolUse": [
      {
        "matcher": "fs_write",
        "command": "cargo fmt --all"
      }
    ]
  }
}
```

### MCP Servers

**Configuring MCP:**

Create `~/.aws/amazonq/mcp.json`:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@github/mcp-server"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

**Using MCP Tools:**

```
> List my open GitHub issues

Q will use the GitHub MCP server to fetch issues
```

### Context References

**File References:**

```
> Explain the code in @src/main.rs

> Review @README.md and suggest improvements
```

**Directory References:**

```
> What files are in @src/components/ ?
```

---

## Best Practices

### 1. Effective Communication

**Be Specific:**

```
# Good
> Add error handling for network timeouts in fetchUserData()

# Less effective
> Fix the error handling
```

**Provide Context:**

```
> This is a Rust project using Tokio. Add a new async function for user lookup.
```

**Use References:**

```
> Update @src/main.rs to add logging
```

### 2. Security

**Review Before Approving:**
- Always review code changes
- Check bash commands before execution
- Be cautious with file deletions
- Verify AWS operations

**Configure Tool Permissions:**

```json
{
  "toolsSettings": {
    "execute_bash": {
      "allowedCommands": ["git status", "cargo test"],
      "deniedCommands": ["rm -rf /", "sudo .*"]
    }
  }
}
```

### 3. Knowledge Management

**Organize by Purpose:**

```
/knowledge add -n "api-docs" -p ./docs/api --index-type Best
/knowledge add -n "source-code" -p ./src --index-type Fast
/knowledge add -n "config" -p ./config --index-type Fast
```

**Regular Cleanup:**

```
> /knowledge show
> /knowledge remove "old-docs"
```

### 4. Agent Design

**Specialized Agents:**

- `aws-expert` - For AWS operations
- `rust-dev` - For Rust development
- `frontend-dev` - For UI/UX work

**Template Agent:**

```json
{
  "name": "project-specific",
  "prompt": "file://./.amazonq/prompt.md",
  "tools": ["fs_read", "fs_write", "execute_bash"],
  "resources": [
    "file://README.md",
    "file://.amazonq/rules/**/*.md"
  ],
  "hooks": {
    "agentSpawn": [
      {
        "command": "git status"
      }
    ]
  }
}
```

---

## Troubleshooting

### Common Issues

**1. Authentication Problems**

```bash
# Check status
q status

# Re-authenticate
q logout
q login
```

**2. Agent Not Found**

```bash
# Check agent location
ls ~/.aws/amazonq/cli-agents/

# Verify agent JSON is valid
jq . ~/.aws/amazonq/cli-agents/my-agent.json
```

**3. Tool Permission Denied**

```
> Check your agent's allowedTools configuration
> Verify toolsSettings for the specific tool
```

**4. Knowledge Base Issues**

```
# Check if enabled
q settings chat.enableKnowledge

# Enable if needed
q settings chat.enableKnowledge true

# Check knowledge status
> /knowledge show
```

**5. MCP Server Not Working**

```bash
# Verify MCP config
q settings --list | grep mcp

# Check server is installed
which <mcp-server-command>

# Test manually
<mcp-server-command> --help
```

### Getting Help

**Within Q CLI:**

```
> /help                    # Show commands
> How do I use X?         # Ask Q
> /issue description      # Report issue
```

**External Resources:**

- AWS Documentation: https://docs.aws.amazon.com/amazonq/
- GitHub Issues: https://github.com/aws/amazon-q-developer-cli/issues

---

## Quick Reference Card

### Essential Commands

| Command | Purpose |
|---------|---------|
| `q chat` | Start session |
| `q chat --agent <name>` | Start with agent |
| `q settings` | Manage settings |
| `/exit` | Quit |
| `/clear` | Clear chat |
| `/help` | Show help |

### Context References

| Syntax | Meaning |
|--------|---------|
| `@file` | Reference file |
| `@dir/` | Reference directory |
| `/command` | Slash command |

### Knowledge Commands

| Command | Action |
|---------|--------|
| `/knowledge add` | Add to knowledge base |
| `/knowledge show` | Show all entries |
| `/knowledge remove` | Remove entry |

### TODO Commands

| Command | Action |
|---------|--------|
| `/todos view` | View lists |
| `/todos resume` | Resume list |
| `/clear-finished` | Clear completed |

---

*For more details, see the [API Reference](./API.md) and [Architecture](./ARCHITECTURE.md) documentation.*
