/**
 * SuperAgent Protocol Enhancement SDK
 * JavaScript/TypeScript client library for SuperAgent Protocol Enhancement API
 *
 * @version 1.0.0
 * @author SuperAgent
 * @license MIT
 */

class SuperAgentClient {
    constructor(baseURL = 'http://localhost:8080', options = {}) {
        this.baseURL = baseURL.replace(/\/$/, '');
        this.timeout = options.timeout || 30000;
        this.apiKey = options.apiKey || null;
        this.headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };

        if (this.apiKey) {
            this.headers['Authorization'] = `Bearer ${this.apiKey}`;
        }
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            method: options.method || 'GET',
            headers: { ...this.headers, ...options.headers },
            signal: AbortSignal.timeout(this.timeout)
        };

        if (options.body) {
            config.body = JSON.stringify(options.body);
        }

        try {
            const response = await fetch(url, config);

            if (!response.ok) {
                const error = await response.text();
                throw new Error(`HTTP ${response.status}: ${error}`);
            }

            return await response.json();
        } catch (error) {
            if (error.name === 'TimeoutError') {
                throw new Error('Request timeout');
            }
            throw error;
        }
    }

    // MCP Protocol Methods
    async mcpCallTool(serverId, toolName, parameters = {}) {
        return this.request('/api/v1/mcp/tools/call', {
            method: 'POST',
            body: { server_id: serverId, tool_name: toolName, parameters }
        });
    }

    async mcpListTools(serverId = null) {
        const params = serverId ? `?server_id=${serverId}` : '';
        return this.request(`/api/v1/mcp/tools/list${params}`);
    }

    async mcpListServers() {
        return this.request('/api/v1/mcp/servers');
    }

    // LSP Protocol Methods
    async lspCompletion(filePath, line, character) {
        return this.request('/api/v1/lsp/completion', {
            method: 'POST',
            body: { file_path: filePath, line, character }
        });
    }

    async lspHover(filePath, line, character) {
        return this.request('/api/v1/lsp/hover', {
            method: 'POST',
            body: { file_path: filePath, line, character }
        });
    }

    async lspDefinition(filePath, line, character) {
        return this.request('/api/v1/lsp/definition', {
            method: 'POST',
            body: { file_path: filePath, line, character }
        });
    }

    async lspDiagnostics(filePath) {
        return this.request(`/api/v1/lsp/diagnostics?file_path=${encodeURIComponent(filePath)}`);
    }

    // ACP Protocol Methods
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

    // Analytics Methods
    async getAnalytics() {
        return this.request('/api/v1/analytics/metrics');
    }

    async getProtocolMetrics(protocol) {
        return this.request(`/api/v1/analytics/metrics/${protocol}`);
    }

    async getHealthStatus() {
        return this.request('/api/v1/analytics/health');
    }

    async recordRequest(protocol, method, duration, success = true, errorType = '') {
        return this.request('/api/v1/analytics/record', {
            method: 'POST',
            body: { protocol, method, duration, success, error_type: errorType }
        });
    }

    // Plugin Methods
    async listPlugins() {
        return this.request('/api/v1/plugins/');
    }

    async loadPlugin(path) {
        return this.request('/api/v1/plugins/load', {
            method: 'POST',
            body: { path }
        });
    }

    async unloadPlugin(pluginId) {
        return this.request(`/api/v1/plugins/${pluginId}`, {
            method: 'DELETE'
        });
    }

    async executePlugin(pluginId, operation, params = {}) {
        return this.request(`/api/v1/plugins/${pluginId}/execute`, {
            method: 'POST',
            body: { operation, params }
        });
    }

    async searchMarketplace(query = '', protocol = '') {
        const params = new URLSearchParams();
        if (query) params.append('q', query);
        if (protocol) params.append('protocol', protocol);
        return this.request(`/api/v1/plugins/marketplace?${params}`);
    }

    async registerPluginInMarketplace(plugin) {
        return this.request('/api/v1/plugins/marketplace/register', {
            method: 'POST',
            body: plugin
        });
    }

    // Template Methods
    async listTemplates(protocol = '') {
        const params = protocol ? `?protocol=${protocol}` : '';
        return this.request(`/api/v1/templates/${params}`);
    }

    async getTemplate(templateId) {
        return this.request(`/api/v1/templates/${templateId}`);
    }

    async generateFromTemplate(templateId, config = {}) {
        return this.request(`/api/v1/templates/${templateId}/generate`, {
            method: 'POST',
            body: { config }
        });
    }

    // System Methods
    async health() {
        return this.request('/api/v1/health');
    }

    async status() {
        return this.request('/api/v1/status');
    }

    async metrics() {
        const response = await fetch(`${this.baseURL}/api/v1/metrics`);
        return response.text();
    }
}

// Utility functions for common operations
class SuperAgentUtils {
    static async createUnifiedClient(baseURL, apiKey) {
        return new SuperAgentClient(baseURL, { apiKey });
    }

    static async initializeDefaultPlugins(client) {
        try {
            // Load default MCP plugin
            await client.loadPlugin('/opt/superagent/plugins/mcp-basic-integration.so');

            // Load default LSP plugin
            await client.loadPlugin('/opt/superagent/plugins/lsp-code-completion.so');

            // Load default ACP plugin
            await client.loadPlugin('/opt/superagent/plugins/acp-agent-orchestration.so');

            console.log('Default plugins loaded successfully');
        } catch (error) {
            console.warn('Some plugins failed to load:', error.message);
        }
    }

    static async setupDevelopmentEnvironment(client) {
        try {
            // Generate default MCP integration from template
            const mcpTemplate = await client.generateFromTemplate('mcp-basic-integration', {
                enabled: true,
                timeout: '30s'
            });

            // Generate default LSP integration
            const lspTemplate = await client.generateFromTemplate('lsp-code-completion', {
                language: 'go',
                enabled: true
            });

            console.log('Development environment templates generated');
            return { mcpTemplate, lspTemplate };
        } catch (error) {
            console.error('Failed to setup development environment:', error);
            throw error;
        }
    }

    static createWorkflowOrchestrator(client) {
        return {
            async executeMCPWorkflow(serverId, operations) {
                const results = [];
                for (const op of operations) {
                    try {
                        const result = await client.mcpCallTool(serverId, op.tool, op.params);
                        results.push({ operation: op, result, success: true });
                    } catch (error) {
                        results.push({ operation: op, error: error.message, success: false });
                    }
                }
                return results;
            },

            async executeLSPWorkflow(filePath, operations) {
                const results = [];
                for (const op of operations) {
                    try {
                        let result;
                        switch (op.type) {
                            case 'completion':
                                result = await client.lspCompletion(filePath, op.line, op.character);
                                break;
                            case 'hover':
                                result = await client.lspHover(filePath, op.line, op.character);
                                break;
                            case 'definition':
                                result = await client.lspDefinition(filePath, op.line, op.character);
                                break;
                            case 'diagnostics':
                                result = await client.lspDiagnostics(filePath);
                                break;
                            default:
                                throw new Error(`Unknown LSP operation: ${op.type}`);
                        }
                        results.push({ operation: op, result, success: true });
                    } catch (error) {
                        results.push({ operation: op, error: error.message, success: false });
                    }
                }
                return results;
            },

            async executeACPWorkflow(agentId, operations) {
                const results = [];
                for (const op of operations) {
                    try {
                        let result;
                        if (op.action) {
                            result = await client.acpExecute(op.action, agentId, op.params);
                        } else if (op.message) {
                            result = await client.acpBroadcast(op.message, op.targets);
                        }
                        results.push({ operation: op, result, success: true });
                    } catch (error) {
                        results.push({ operation: op, error: error.message, success: false });
                    }
                }
                return results;
            }
        };
    }

    static createAnalyticsMonitor(client, interval = 30000) {
        let monitoring = false;

        return {
            start() {
                if (monitoring) return;
                monitoring = true;

                const monitor = async () => {
                    if (!monitoring) return;

                    try {
                        const status = await client.getHealthStatus();
                        const metrics = await client.getAnalytics();

                        // Emit monitoring event
                        if (typeof window !== 'undefined' && window.dispatchEvent) {
                            window.dispatchEvent(new CustomEvent('superagent:metrics', {
                                detail: { status, metrics }
                            }));
                        }

                        console.log('System health:', status.overall_status);
                        console.log('Active protocols:', metrics.total_protocols);
                    } catch (error) {
                        console.error('Monitoring error:', error);
                    }

                    setTimeout(monitor, interval);
                };

                monitor();
            },

            stop() {
                monitoring = false;
            },

            async getReport() {
                const [analytics, health] = await Promise.all([
                    client.getAnalytics(),
                    client.getHealthStatus()
                ]);

                return {
                    timestamp: new Date().toISOString(),
                    analytics,
                    health,
                    summary: {
                        total_requests: analytics.summary?.total_requests || 0,
                        error_rate: analytics.summary?.error_rate || 0,
                        system_health: health.overall_status
                    }
                };
            }
        };
    }
}

// Export for different environments
if (typeof module !== 'undefined' && module.exports) {
    // CommonJS
    module.exports = { SuperAgentClient, SuperAgentUtils };
} else if (typeof define === 'function' && define.amd) {
    // AMD
    define([], function() {
        return { SuperAgentClient, SuperAgentUtils };
    });
} else if (typeof window !== 'undefined') {
    // Browser global
    window.SuperAgent = { SuperAgentClient, SuperAgentUtils };
}