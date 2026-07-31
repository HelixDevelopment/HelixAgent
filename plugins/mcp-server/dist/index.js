#!/usr/bin/env node
"use strict";
/**
 * HelixAgent Generic MCP Server
 *
 * Provides MCP protocol support for Tier 2-3 CLI agents that don't have
 * rich plugin systems. Supports stdio and SSE transports.
 */
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __exportStar = (this && this.__exportStar) || function(m, exports) {
    for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports, p)) __createBinding(exports, m, p);
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.isHostAllowed = exports.parseAllowedHosts = exports.originHeaders = exports.parseAllowedOrigins = exports.HelixAgentTransport = exports.HelixAgentMCPServer = void 0;
exports.envConfig = envConfig;
const transport_1 = require("./transport");
const cors_1 = require("./cors");
const host_1 = require("./host");
const tools_1 = require("./tools");
const defaultConfig = {
    endpoint: 'https://localhost:7061',
    transport: 'stdio',
    preferHTTP3: true,
    enableTOON: true,
    enableBrotli: true,
    allowedOrigins: [],
    allowedHosts: [],
};
/**
 * Read configuration from the process environment.
 *
 * Container definitions (Dockerfile `ENV`, compose `environment:`) set
 * MCP_PORT / MCP_TRANSPORT / HELIXAGENT_URL to steer this server. Before this
 * function existed the code read NOTHING from the environment, so every one of
 * those settings was inert: the published container port and the declared
 * container health check were both written to match MCP_PORT while the server
 * silently ignored it. That made the health check unsatisfiable — it probed a
 * port nothing could ever listen on — and made the port knob look like
 * configuration while doing nothing at all.
 *
 * Precedence is defaults < environment < CLI flags (applied in `main()`), so a
 * CLI user's explicit `--port` / `--transport` always wins, and an embedder
 * constructing `HelixAgentMCPServer` programmatically is never surprised by
 * ambient environment — env is applied only on the CLI entry path.
 *
 * An env var that is SET but INVALID is a hard error, never a silent fallback:
 * silently ignoring a bad value is the exact defect this function exists to
 * remove (a setting that produces "no change and no error").
 */
function envConfig(env = process.env) {
    const config = {};
    const rawPort = env.MCP_PORT?.trim();
    if (rawPort) {
        const port = Number(rawPort);
        if (!Number.isInteger(port) || port < 1 || port > 65535) {
            throw new Error(`MCP_PORT must be an integer between 1 and 65535, got '${rawPort}'`);
        }
        config.port = port;
    }
    const rawTransport = env.MCP_TRANSPORT?.trim();
    if (rawTransport) {
        if (rawTransport !== 'stdio' && rawTransport !== 'sse') {
            throw new Error(`MCP_TRANSPORT must be 'stdio' or 'sse', got '${rawTransport}'`);
        }
        config.transport = rawTransport;
    }
    const rawEndpoint = env.HELIXAGENT_URL?.trim();
    if (rawEndpoint) {
        config.endpoint = rawEndpoint;
    }
    // Cross-origin allowlist for the SSE transport (HXC-212), comma-separated.
    // Unset means EMPTY, i.e. default-deny: no cross-origin browser caller may
    // read a response. Unlike MCP_PORT / MCP_TRANSPORT an unparseable value is
    // not possible here -- a literal "*" and blank fields are dropped by
    // parseAllowedOrigins rather than rejected, because the safe reading of "*"
    // is "deny", and failing closed is never the surprise that failing open is.
    const rawOrigins = env.MCP_ALLOWED_ORIGINS;
    if (rawOrigins !== undefined) {
        config.allowedOrigins = (0, cors_1.parseAllowedOrigins)(rawOrigins);
    }
    // DNS names this deployment is reached by (HXC-172), comma-separated. Unset
    // means EMPTY, which is already safe: loopback and IP-literal Hosts are
    // permitted unconditionally, every other DNS name is refused. Like the origin
    // list a literal "*" is dropped rather than rejected -- the safe reading of
    // "*" is "add nothing", and failing closed is never the surprise that failing
    // open is.
    const rawHosts = env.MCP_ALLOWED_HOSTS;
    if (rawHosts !== undefined) {
        config.allowedHosts = (0, host_1.parseAllowedHosts)(rawHosts);
    }
    // Boolean transport toggles. Only an explicit 'false'/'0' disables them, so an
    // unset or malformed value keeps the built-in default rather than silently
    // flipping behaviour.
    const boolEnv = (raw) => {
        const value = raw?.trim().toLowerCase();
        if (!value)
            return undefined;
        if (value === 'false' || value === '0' || value === 'no')
            return false;
        if (value === 'true' || value === '1' || value === 'yes')
            return true;
        return undefined;
    };
    const http3 = boolEnv(env.ENABLE_HTTP3);
    if (http3 !== undefined)
        config.preferHTTP3 = http3;
    const toon = boolEnv(env.ENABLE_TOON);
    if (toon !== undefined)
        config.enableTOON = toon;
    const brotli = boolEnv(env.ENABLE_BROTLI);
    if (brotli !== undefined)
        config.enableBrotli = brotli;
    return config;
}
/**
 * Generic MCP Server for HelixAgent
 */
class HelixAgentMCPServer {
    config;
    transport;
    tools;
    constructor(config) {
        this.config = { ...defaultConfig, ...config };
        this.transport = new transport_1.HelixAgentTransport(this.config.endpoint, {
            preferHTTP3: this.config.preferHTTP3,
            enableTOON: this.config.enableTOON,
            enableBrotli: this.config.enableBrotli,
        });
        // Register tools - Core functionality
        this.tools = new Map();
        this.tools.set('helixagent_debate', new tools_1.DebateTool(this.transport));
        this.tools.set('helixagent_ensemble', new tools_1.EnsembleTool(this.transport));
        this.tools.set('helixagent_task', new tools_1.TaskTool(this.transport));
        this.tools.set('helixagent_rag', new tools_1.RAGTool(this.transport));
        this.tools.set('helixagent_memory', new tools_1.MemoryTool(this.transport));
        this.tools.set('helixagent_providers', new tools_1.ProvidersTool(this.transport));
        // Register tools - Protocol tools (ACP, LSP, Embeddings, Vision, Cognee)
        this.tools.set('helixagent_acp', new tools_1.ACPTool(this.transport));
        this.tools.set('helixagent_lsp', new tools_1.LSPTool(this.transport));
        this.tools.set('helixagent_embeddings', new tools_1.EmbeddingsTool(this.transport));
        this.tools.set('helixagent_vision', new tools_1.VisionTool(this.transport));
        this.tools.set('helixagent_cognee', new tools_1.CogneeTool(this.transport));
    }
    /**
     * Start the MCP server
     */
    async start() {
        // Connect to HelixAgent
        await this.transport.connect();
        if (this.config.transport === 'stdio') {
            await this.runStdio();
        }
        else {
            await this.runSSE();
        }
    }
    /**
     * Run in stdio mode
     */
    async runStdio() {
        const readline = await import('readline');
        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
            terminal: false,
        });
        rl.on('line', async (line) => {
            if (!line.trim())
                return;
            try {
                const request = JSON.parse(line);
                const response = await this.handleRequest(request);
                if (response) {
                    process.stdout.write(JSON.stringify(response) + '\n');
                }
            }
            catch (error) {
                const errorResponse = {
                    jsonrpc: '2.0',
                    id: 0,
                    error: {
                        code: -32700,
                        message: 'Parse error',
                    },
                };
                process.stdout.write(JSON.stringify(errorResponse) + '\n');
            }
        });
        rl.on('close', () => {
            process.exit(0);
        });
    }
    /**
     * Run in SSE mode
     */
    async runSSE() {
        const http = await import('http');
        const port = this.config.port || 7062;
        // HXC-212: the origin allowlist, resolved once. Empty = default-deny.
        // Re-normalised here (not trusted as given) so a programmatic embedder
        // cannot smuggle a literal "*" past the parser.
        const allowedOrigins = new Set((0, cors_1.parseAllowedOrigins)(this.config.allowedOrigins));
        // HXC-172: DNS-rebinding protection. Re-normalised here (not trusted as
        // given) so a programmatic embedder cannot smuggle a literal "*" past the
        // parser, exactly as with the origin list above.
        const allowedHosts = new Set((0, host_1.parseAllowedHosts)(this.config.allowedHosts));
        const server = http.createServer(async (req, res) => {
            // HXC-172, defence 1 of 2 — Host validation, applied before anything else
            // and to every route. `server.listen(port)` binds all interfaces, so a
            // page that rebinds its own DNS name to this address would otherwise
            // reach the full MCP surface. Loopback and IP-literal Hosts are always
            // permitted, so the container health check and host-side access need no
            // configuration; any other DNS name must be named in MCP_ALLOWED_HOSTS.
            if (!(0, host_1.isHostAllowed)(req.headers.host, allowedHosts)) {
                res.writeHead(403, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({
                    jsonrpc: '2.0',
                    id: 0,
                    error: { code: -32600, message: 'Host not allowed' },
                }));
                return;
            }
            // Never "*": only an origin on the allowlist is echoed back, and only to
            // itself. A request with no Origin gets no Allow-Origin header and is
            // otherwise untouched.
            const corsHeaders = (0, cors_1.originHeaders)(req.headers.origin, allowedOrigins);
            // HXC-172, defence 2 of 2 — refuse a non-allowlisted browser origin
            // OUTRIGHT rather than merely withholding Allow-Origin.
            //
            // HXC-212 stopped a hostile page READING our responses, which is what
            // CORS governs. It does not stop the request being EXECUTED: the browser
            // discards the reply only after we have already run the call, so a
            // `tools/call` from a hostile page still had its side effect. Blocking
            // the read is not blocking the call.
            //
            // OPTIONS is deliberately exempt: the preflight is the browser ASKING
            // permission, and answering it with 204-and-no-Allow-Origin is already
            // the correct denial (the browser then never sends the real request).
            // Rejecting the preflight instead would change a settled contract
            // (§11.4.120) while closing nothing extra.
            const origin = req.headers.origin;
            if (req.method !== 'OPTIONS' && origin !== undefined && !allowedOrigins.has(origin)) {
                res.writeHead(403, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({
                    jsonrpc: '2.0',
                    id: 0,
                    error: { code: -32600, message: 'Origin not allowed' },
                }));
                return;
            }
            if (req.method === 'OPTIONS') {
                // The 204 short-circuit and the advertised methods/headers are
                // unchanged; only WHO may read a response changed.
                res.writeHead(204, {
                    ...corsHeaders,
                    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
                    'Access-Control-Allow-Headers': 'Content-Type',
                });
                res.end();
                return;
            }
            if (req.url === '/health') {
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ status: 'healthy' }));
                return;
            }
            if (req.method === 'POST' && req.url === '/mcp') {
                let body = '';
                req.on('data', (chunk) => { body += chunk; });
                req.on('end', async () => {
                    try {
                        const request = JSON.parse(body);
                        const response = await this.handleRequest(request);
                        res.writeHead(200, {
                            'Content-Type': 'application/json',
                            ...corsHeaders,
                        });
                        res.end(JSON.stringify(response));
                    }
                    catch (error) {
                        res.writeHead(400, { 'Content-Type': 'application/json' });
                        res.end(JSON.stringify({
                            jsonrpc: '2.0',
                            id: 0,
                            error: { code: -32700, message: 'Parse error' },
                        }));
                    }
                });
                return;
            }
            res.writeHead(404);
            res.end('Not found');
        });
        server.listen(port, () => {
            console.error(`HelixAgent MCP Server running on http://localhost:${port}`);
        });
    }
    /**
     * Handle MCP request
     */
    async handleRequest(request) {
        switch (request.method) {
            case 'initialize':
                return this.handleInitialize(request);
            case 'notifications/initialized':
                return null; // No response for notifications
            case 'tools/list':
                return this.handleListTools(request);
            case 'tools/call':
                return this.handleCallTool(request);
            case 'resources/list':
                return this.handleListResources(request);
            case 'resources/read':
                return this.handleReadResource(request);
            default:
                return {
                    jsonrpc: '2.0',
                    id: request.id,
                    error: {
                        code: -32601,
                        message: `Method not found: ${request.method}`,
                    },
                };
        }
    }
    /**
     * Handle initialize
     */
    handleInitialize(request) {
        return {
            jsonrpc: '2.0',
            id: request.id,
            result: {
                protocolVersion: '2024-11-05',
                capabilities: {
                    tools: { listChanged: true },
                    resources: { subscribe: false, listChanged: false },
                },
                serverInfo: {
                    name: 'helixagent-mcp-server',
                    version: '1.0.0',
                },
            },
        };
    }
    /**
     * Handle tools/list
     */
    handleListTools(request) {
        const tools = Array.from(this.tools.entries()).map(([name, tool]) => ({
            name,
            description: tool.description,
            inputSchema: tool.inputSchema,
        }));
        return {
            jsonrpc: '2.0',
            id: request.id,
            result: { tools },
        };
    }
    /**
     * Handle tools/call
     */
    async handleCallTool(request) {
        const params = request.params;
        const toolName = params?.name;
        const toolArgs = params?.arguments || {};
        const tool = this.tools.get(toolName);
        if (!tool) {
            return {
                jsonrpc: '2.0',
                id: request.id,
                error: {
                    code: -32602,
                    message: `Unknown tool: ${toolName}`,
                },
            };
        }
        try {
            const result = await tool.execute(toolArgs);
            return {
                jsonrpc: '2.0',
                id: request.id,
                result: {
                    content: [
                        {
                            type: 'text',
                            text: typeof result === 'string' ? result : JSON.stringify(result, null, 2),
                        },
                    ],
                },
            };
        }
        catch (error) {
            return {
                jsonrpc: '2.0',
                id: request.id,
                error: {
                    code: -32000,
                    message: error instanceof Error ? error.message : 'Tool execution failed',
                },
            };
        }
    }
    /**
     * Handle resources/list
     */
    handleListResources(request) {
        return {
            jsonrpc: '2.0',
            id: request.id,
            result: {
                resources: [
                    {
                        uri: 'helixagent://providers',
                        name: 'HelixAgent Providers',
                        description: 'List of available LLM providers and their status',
                        mimeType: 'application/json',
                    },
                    {
                        uri: 'helixagent://debates',
                        name: 'Active Debates',
                        description: 'List of currently active AI debates',
                        mimeType: 'application/json',
                    },
                    {
                        uri: 'helixagent://tasks',
                        name: 'Background Tasks',
                        description: 'List of background tasks and their status',
                        mimeType: 'application/json',
                    },
                ],
            },
        };
    }
    /**
     * Handle resources/read
     */
    async handleReadResource(request) {
        const params = request.params;
        const uri = params?.uri;
        try {
            let content;
            switch (uri) {
                case 'helixagent://providers':
                    content = await this.transport.request('GET', '/v1/providers');
                    break;
                case 'helixagent://debates':
                    content = await this.transport.request('GET', '/v1/debates');
                    break;
                case 'helixagent://tasks':
                    content = await this.transport.request('GET', '/v1/tasks');
                    break;
                default:
                    return {
                        jsonrpc: '2.0',
                        id: request.id,
                        error: {
                            code: -32602,
                            message: `Unknown resource: ${uri}`,
                        },
                    };
            }
            return {
                jsonrpc: '2.0',
                id: request.id,
                result: {
                    contents: [
                        {
                            uri,
                            mimeType: 'application/json',
                            text: JSON.stringify(content, null, 2),
                        },
                    ],
                },
            };
        }
        catch (error) {
            return {
                jsonrpc: '2.0',
                id: request.id,
                error: {
                    code: -32000,
                    message: error instanceof Error ? error.message : 'Resource read failed',
                },
            };
        }
    }
}
exports.HelixAgentMCPServer = HelixAgentMCPServer;
// CLI entry point
async function main() {
    const args = process.argv.slice(2);
    // Precedence: built-in defaults < environment < CLI flags. Env is read here
    // (the CLI entry path) rather than in the constructor so programmatic
    // embedders are not affected by ambient environment.
    const config = envConfig();
    for (let i = 0; i < args.length; i++) {
        switch (args[i]) {
            case '--endpoint':
                config.endpoint = args[++i];
                break;
            case '--port':
                config.port = parseInt(args[++i], 10);
                break;
            case '--transport':
                config.transport = args[++i];
                break;
            case '--no-http3':
                config.preferHTTP3 = false;
                break;
            case '--no-toon':
                config.enableTOON = false;
                break;
            case '--no-brotli':
                config.enableBrotli = false;
                break;
            case '--help':
                console.log(`
HelixAgent MCP Server

Usage: helixagent-mcp [options]

Options:
  --endpoint <url>     HelixAgent endpoint (default: https://localhost:7061)
  --port <port>        SSE server port (default: 7062)
  --transport <type>   Transport type: stdio or sse (default: stdio)
  --no-http3           Disable HTTP/3 preference
  --no-toon            Disable TOON protocol
  --no-brotli          Disable Brotli compression
  --help               Show this help message

Environment (overridden by the flags above, overrides the defaults):
  MCP_PORT             SSE server port
  MCP_TRANSPORT        Transport type: stdio or sse
  HELIXAGENT_URL       HelixAgent endpoint
  MCP_ALLOWED_ORIGINS  Comma-separated origins permitted to read SSE-transport
                       responses cross-origin. Unset = deny all cross-origin
                       browser access. A literal "*" is rejected. Clients that
                       send no Origin (the CLI agents) are unaffected. A request
                       from an origin that is not on the list is refused with
                       403, not merely made unreadable.
  MCP_ALLOWED_HOSTS    Comma-separated DNS names this server may be addressed
                       by (DNS-rebinding protection). Loopback and IP-literal
                       Host values are always permitted, so the default empty
                       list is already safe and needs no configuration; set this
                       only when a deployment is reached by a DNS name.
  ENABLE_HTTP3         false/0/no disables HTTP/3 preference
  ENABLE_TOON          false/0/no disables TOON protocol
  ENABLE_BROTLI        false/0/no disables Brotli compression

Note: the SSE transport is what opens a listening TCP port and serves /health;
under the stdio transport no port is bound. A container that needs its port
published or health-checked must set MCP_TRANSPORT=sse.
        `);
                process.exit(0);
        }
    }
    const server = new HelixAgentMCPServer(config);
    await server.start();
}
// Export for programmatic use
var transport_2 = require("./transport");
Object.defineProperty(exports, "HelixAgentTransport", { enumerable: true, get: function () { return transport_2.HelixAgentTransport; } });
var cors_2 = require("./cors");
Object.defineProperty(exports, "parseAllowedOrigins", { enumerable: true, get: function () { return cors_2.parseAllowedOrigins; } });
Object.defineProperty(exports, "originHeaders", { enumerable: true, get: function () { return cors_2.originHeaders; } });
var host_2 = require("./host");
Object.defineProperty(exports, "parseAllowedHosts", { enumerable: true, get: function () { return host_2.parseAllowedHosts; } });
Object.defineProperty(exports, "isHostAllowed", { enumerable: true, get: function () { return host_2.isHostAllowed; } });
__exportStar(require("./tools"), exports);
// Run if invoked directly
if (require.main === module) {
    main().catch((error) => {
        console.error('MCP Server error:', error);
        process.exit(1);
    });
}
//# sourceMappingURL=index.js.map