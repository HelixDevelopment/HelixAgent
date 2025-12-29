#!/usr/bin/env node

/**
 * SuperAgent Protocol Enhancement CLI
 * Command-line interface for SuperAgent Protocol Enhancement
 *
 * @version 1.0.0
 * @author SuperAgent
 * @license MIT
 */

const https = require('https');
const http = require('http');
const { URL } = require('url');

class SuperAgentCLI {
    constructor() {
        this.baseURL = process.env.SUPERAGENT_URL || 'http://localhost:8080';
        this.apiKey = process.env.SUPERAGENT_API_KEY || null;
        this.timeout = 30000;
    }

    async request(endpoint, options = {}) {
        const url = new URL(endpoint, this.baseURL);
        const config = {
            method: options.method || 'GET',
            headers: {
                'Content-Type': 'application/json',
                'User-Agent': 'SuperAgent-CLI/1.0.0'
            },
            timeout: this.timeout
        };

        if (this.apiKey) {
            config.headers['Authorization'] = `Bearer ${this.apiKey}`;
        }

        if (options.headers) {
            Object.assign(config.headers, options.headers);
        }

        if (options.body) {
            config.body = JSON.stringify(options.body);
        }

        const protocol = url.protocol === 'https:' ? https : http;

        return new Promise((resolve, reject) => {
            const req = protocol.request(url, config, (res) => {
                let data = '';

                res.on('data', (chunk) => {
                    data += chunk;
                });

                res.on('end', () => {
                    try {
                        if (res.statusCode >= 200 && res.statusCode < 300) {
                            const result = data ? JSON.parse(data) : {};
                            resolve(result);
                        } else {
                            reject(new Error(`HTTP ${res.statusCode}: ${data}`));
                        }
                    } catch (error) {
                        reject(new Error(`Parse error: ${error.message}`));
                    }
                });
            });

            req.on('error', (error) => {
                reject(error);
            });

            req.on('timeout', () => {
                req.destroy();
                reject(new Error('Request timeout'));
            });

            if (options.body) {
                req.write(JSON.stringify(options.body));
            }

            req.end();
        });
    }

    // MCP Commands
    async mcpListTools(serverId = null) {
        const params = serverId ? `?server_id=${serverId}` : '';
        return this.request(`/api/v1/mcp/tools/list${params}`);
    }

    async mcpCallTool(serverId, toolName, parameters = {}) {
        return this.request('/api/v1/mcp/tools/call', {
            method: 'POST',
            body: { server_id: serverId, tool_name: toolName, parameters }
        });
    }

    async mcpListServers() {
        return this.request('/api/v1/mcp/servers');
    }

    // LSP Commands
    async lspCompletion(filePath, line, character) {
        return this.request('/api/v1/lsp/completion', {
            method: 'POST',
            body: { file_path: filePath, line: parseInt(line), character: parseInt(character) }
        });
    }

    async lspHover(filePath, line, character) {
        return this.request('/api/v1/lsp/hover', {
            method: 'POST',
            body: { file_path: filePath, line: parseInt(line), character: parseInt(character) }
        });
    }

    async lspDefinition(filePath, line, character) {
        return this.request('/api/v1/lsp/definition', {
            method: 'POST',
            body: { file_path: filePath, line: parseInt(line), character: parseInt(character) }
        });
    }

    async lspDiagnostics(filePath) {
        return this.request(`/api/v1/lsp/diagnostics?file_path=${encodeURIComponent(filePath)}`);
    }

    // ACP Commands
    async acpExecute(action, agentId = 'default', params = {}) {
        return this.request('/api/v1/acp/execute', {
            method: 'POST',
            body: { action, agent_id: agentId, params }
        });
    }

    async acpBroadcast(message, targets = []) {
        return this.request('/api/v1/acp/broadcast', {
            method: 'POST',
            body: { message, targets }
        });
    }

    async acpStatus(agentId = null) {
        const params = agentId ? `?agent_id=${agentId}` : '';
        return this.request(`/api/v1/acp/status${params}`);
    }

    // Analytics Commands
    async analytics() {
        return this.request('/api/v1/analytics/metrics');
    }

    async analyticsProtocol(protocol) {
        return this.request(`/api/v1/analytics/metrics/${protocol}`);
    }

    async analyticsHealth() {
        return this.request('/api/v1/analytics/health');
    }

    // Plugin Commands
    async plugins() {
        return this.request('/api/v1/plugins/');
    }

    async pluginLoad(path) {
        return this.request('/api/v1/plugins/load', {
            method: 'POST',
            body: { path }
        });
    }

    async pluginUnload(pluginId) {
        return this.request(`/api/v1/plugins/${pluginId}`, {
            method: 'DELETE'
        });
    }

    async pluginExecute(pluginId, operation, params = {}) {
        return this.request(`/api/v1/plugins/${pluginId}/execute`, {
            method: 'POST',
            body: { operation, params }
        });
    }

    async pluginMarketplace(query = '', protocol = '') {
        const searchParams = new URLSearchParams();
        if (query) searchParams.append('q', query);
        if (protocol) searchParams.append('protocol', protocol);
        return this.request(`/api/v1/plugins/marketplace?${searchParams}`);
    }

    // Template Commands
    async templates(protocol = '') {
        const params = protocol ? `?protocol=${protocol}` : '';
        return this.request(`/api/v1/templates/${params}`);
    }

    async templateGet(templateId) {
        return this.request(`/api/v1/templates/${templateId}`);
    }

    async templateGenerate(templateId, config = {}) {
        return this.request(`/api/v1/templates/${templateId}/generate`, {
            method: 'POST',
            body: { config }
        });
    }

    // System Commands
    async health() {
        return this.request('/api/v1/health');
    }

    async status() {
        return this.request('/api/v1/status');
    }

    async metrics() {
        return this.request('/api/v1/metrics');
    }
}

// CLI Interface
function printUsage() {
    console.log(`
SuperAgent Protocol Enhancement CLI v1.0.0

USAGE:
    superagent-cli <command> [options]

COMMANDS:

  MCP Protocol:
    mcp:tools [server_id]           List MCP tools
    mcp:call <server_id> <tool>     Call MCP tool
    mcp:servers                     List MCP servers

  LSP Protocol:
    lsp:completion <file> <line> <char>  Get code completions
    lsp:hover <file> <line> <char>       Get hover information
    lsp:definition <file> <line> <char>  Get definition location
    lsp:diagnostics <file>               Get file diagnostics

  ACP Protocol:
    acp:execute <action> [agent_id]     Execute action on agent
    acp:broadcast <message> <targets>   Broadcast message to agents
    acp:status [agent_id]               Get agent status

  Analytics:
    analytics                          Get all analytics
    analytics:protocol <protocol>      Get protocol-specific analytics
    analytics:health                   Get system health status

  Plugins:
    plugins                            List loaded plugins
    plugins:load <path>                Load plugin from path
    plugins:unload <plugin_id>         Unload plugin
    plugins:execute <id> <op> [params] Execute plugin operation
    plugins:marketplace [query] [protocol] Search plugin marketplace

  Templates:
    templates [protocol]               List templates
    templates:get <template_id>        Get template details
    templates:generate <id> [config]   Generate from template

  System:
    health                             System health check
    status                             System status
    metrics                            Prometheus metrics

ENVIRONMENT VARIABLES:
    SUPERAGENT_URL        API server URL (default: http://localhost:8080)
    SUPERAGENT_API_KEY    API key for authentication

EXAMPLES:
    superagent-cli mcp:tools
    superagent-cli lsp:completion main.go 10 5
    superagent-cli acp:execute "process_data" agent-001
    superagent-cli analytics
    superagent-cli plugins:marketplace "mcp"
    superagent-cli templates mcp
`);
}

async function main() {
    const args = process.argv.slice(2);
    if (args.length === 0) {
        printUsage();
        process.exit(1);
    }

    const cli = new SuperAgentCLI();
    const command = args[0];

    try {
        let result;

        switch (command) {
            // MCP commands
            case 'mcp:tools':
                result = await cli.mcpListTools(args[1]);
                break;
            case 'mcp:call':
                if (args.length < 3) throw new Error('Usage: mcp:call <server_id> <tool>');
                result = await cli.mcpCallTool(args[1], args[2], args.slice(3));
                break;
            case 'mcp:servers':
                result = await cli.mcpListServers();
                break;

            // LSP commands
            case 'lsp:completion':
                if (args.length < 4) throw new Error('Usage: lsp:completion <file> <line> <char>');
                result = await cli.lspCompletion(args[1], args[2], args[3]);
                break;
            case 'lsp:hover':
                if (args.length < 4) throw new Error('Usage: lsp:hover <file> <line> <char>');
                result = await cli.lspHover(args[1], args[2], args[3]);
                break;
            case 'lsp:definition':
                if (args.length < 4) throw new Error('Usage: lsp:definition <file> <line> <char>');
                result = await cli.lspDefinition(args[1], args[2], args[3]);
                break;
            case 'lsp:diagnostics':
                if (args.length < 2) throw new Error('Usage: lsp:diagnostics <file>');
                result = await cli.lspDiagnostics(args[1]);
                break;

            // ACP commands
            case 'acp:execute':
                if (args.length < 2) throw new Error('Usage: acp:execute <action> [agent_id]');
                result = await cli.acpExecute(args[1], args[2] || 'default');
                break;
            case 'acp:broadcast':
                if (args.length < 3) throw new Error('Usage: acp:broadcast <message> <targets>');
                result = await cli.acpBroadcast(args[1], args.slice(2));
                break;
            case 'acp:status':
                result = await cli.acpStatus(args[1]);
                break;

            // Analytics commands
            case 'analytics':
                result = await cli.analytics();
                break;
            case 'analytics:protocol':
                if (args.length < 2) throw new Error('Usage: analytics:protocol <protocol>');
                result = await cli.analyticsProtocol(args[1]);
                break;
            case 'analytics:health':
                result = await cli.analyticsHealth();
                break;

            // Plugin commands
            case 'plugins':
                result = await cli.plugins();
                break;
            case 'plugins:load':
                if (args.length < 2) throw new Error('Usage: plugins:load <path>');
                result = await cli.pluginLoad(args[1]);
                break;
            case 'plugins:unload':
                if (args.length < 2) throw new Error('Usage: plugins:unload <plugin_id>');
                result = await cli.pluginUnload(args[1]);
                break;
            case 'plugins:execute':
                if (args.length < 3) throw new Error('Usage: plugins:execute <id> <op> [params]');
                result = await cli.pluginExecute(args[1], args[2], args.slice(3));
                break;
            case 'plugins:marketplace':
                result = await cli.pluginMarketplace(args[1], args[2]);
                break;

            // Template commands
            case 'templates':
                result = await cli.templates(args[1]);
                break;
            case 'templates:get':
                if (args.length < 2) throw new Error('Usage: templates:get <template_id>');
                result = await cli.templateGet(args[1]);
                break;
            case 'templates:generate':
                if (args.length < 2) throw new Error('Usage: templates:generate <id> [config]');
                result = await cli.templateGenerate(args[1], args[2] ? JSON.parse(args[2]) : {});
                break;

            // System commands
            case 'health':
                result = await cli.health();
                break;
            case 'status':
                result = await cli.status();
                break;
            case 'metrics':
                result = await cli.metrics();
                console.log(result);
                return;

            default:
                throw new Error(`Unknown command: ${command}`);
        }

        console.log(JSON.stringify(result, null, 2));

    } catch (error) {
        console.error(`Error: ${error.message}`);
        process.exit(1);
    }
}

if (require.main === module) {
    main().catch(error => {
        console.error(`Fatal error: ${error.message}`);
        process.exit(1);
    });
}

module.exports = SuperAgentCLI;