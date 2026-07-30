#!/usr/bin/env node
/**
 * HelixAgent Generic MCP Server
 *
 * Provides MCP protocol support for Tier 2-3 CLI agents that don't have
 * rich plugin systems. Supports stdio and SSE transports.
 */

import { createReadStream, createWriteStream } from 'fs';
import { HelixAgentTransport } from './transport';
import { DebateTool, EnsembleTool, TaskTool, RAGTool, MemoryTool, ProvidersTool, ACPTool, LSPTool, EmbeddingsTool, VisionTool, CogneeTool } from './tools';

// Configuration
export interface MCPServerConfig {
  endpoint: string;
  transport: 'stdio' | 'sse';
  port?: number;
  preferHTTP3?: boolean;
  enableTOON?: boolean;
  enableBrotli?: boolean;
}

const defaultConfig: MCPServerConfig = {
  endpoint: 'https://localhost:7061',
  transport: 'stdio',
  preferHTTP3: true,
  enableTOON: true,
  enableBrotli: true,
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
export function envConfig(env: NodeJS.ProcessEnv = process.env): Partial<MCPServerConfig> {
  const config: Partial<MCPServerConfig> = {};

  const rawPort = env.MCP_PORT?.trim();
  if (rawPort) {
    const port = Number(rawPort);
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error(
        `MCP_PORT must be an integer between 1 and 65535, got '${rawPort}'`
      );
    }
    config.port = port;
  }

  const rawTransport = env.MCP_TRANSPORT?.trim();
  if (rawTransport) {
    if (rawTransport !== 'stdio' && rawTransport !== 'sse') {
      throw new Error(
        `MCP_TRANSPORT must be 'stdio' or 'sse', got '${rawTransport}'`
      );
    }
    config.transport = rawTransport;
  }

  const rawEndpoint = env.HELIXAGENT_URL?.trim();
  if (rawEndpoint) {
    config.endpoint = rawEndpoint;
  }

  // Boolean transport toggles. Only an explicit 'false'/'0' disables them, so an
  // unset or malformed value keeps the built-in default rather than silently
  // flipping behaviour.
  const boolEnv = (raw: string | undefined): boolean | undefined => {
    const value = raw?.trim().toLowerCase();
    if (!value) return undefined;
    if (value === 'false' || value === '0' || value === 'no') return false;
    if (value === 'true' || value === '1' || value === 'yes') return true;
    return undefined;
  };

  const http3 = boolEnv(env.ENABLE_HTTP3);
  if (http3 !== undefined) config.preferHTTP3 = http3;

  const toon = boolEnv(env.ENABLE_TOON);
  if (toon !== undefined) config.enableTOON = toon;

  const brotli = boolEnv(env.ENABLE_BROTLI);
  if (brotli !== undefined) config.enableBrotli = brotli;

  return config;
}

// MCP Protocol types
interface MCPRequest {
  jsonrpc: '2.0';
  id: string | number;
  method: string;
  params?: unknown;
}

interface MCPResponse {
  jsonrpc: '2.0';
  id: string | number;
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
}

/**
 * Generic MCP Server for HelixAgent
 */
export class HelixAgentMCPServer {
  private config: MCPServerConfig;
  private transport: HelixAgentTransport;
  private tools: Map<string, MCPTool>;

  constructor(config?: Partial<MCPServerConfig>) {
    this.config = { ...defaultConfig, ...config };
    this.transport = new HelixAgentTransport(this.config.endpoint, {
      preferHTTP3: this.config.preferHTTP3,
      enableTOON: this.config.enableTOON,
      enableBrotli: this.config.enableBrotli,
    });

    // Register tools - Core functionality
    this.tools = new Map();
    this.tools.set('helixagent_debate', new DebateTool(this.transport));
    this.tools.set('helixagent_ensemble', new EnsembleTool(this.transport));
    this.tools.set('helixagent_task', new TaskTool(this.transport));
    this.tools.set('helixagent_rag', new RAGTool(this.transport));
    this.tools.set('helixagent_memory', new MemoryTool(this.transport));
    this.tools.set('helixagent_providers', new ProvidersTool(this.transport));

    // Register tools - Protocol tools (ACP, LSP, Embeddings, Vision, Cognee)
    this.tools.set('helixagent_acp', new ACPTool(this.transport));
    this.tools.set('helixagent_lsp', new LSPTool(this.transport));
    this.tools.set('helixagent_embeddings', new EmbeddingsTool(this.transport));
    this.tools.set('helixagent_vision', new VisionTool(this.transport));
    this.tools.set('helixagent_cognee', new CogneeTool(this.transport));
  }

  /**
   * Start the MCP server
   */
  async start(): Promise<void> {
    // Connect to HelixAgent
    await this.transport.connect();

    if (this.config.transport === 'stdio') {
      await this.runStdio();
    } else {
      await this.runSSE();
    }
  }

  /**
   * Run in stdio mode
   */
  private async runStdio(): Promise<void> {
    const readline = await import('readline');
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
      terminal: false,
    });

    rl.on('line', async (line) => {
      if (!line.trim()) return;

      try {
        const request = JSON.parse(line) as MCPRequest;
        const response = await this.handleRequest(request);
        if (response) {
          process.stdout.write(JSON.stringify(response) + '\n');
        }
      } catch (error) {
        const errorResponse: MCPResponse = {
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
  private async runSSE(): Promise<void> {
    const http = await import('http');
    const port = this.config.port || 7062;

    const server = http.createServer(async (req, res) => {
      if (req.method === 'OPTIONS') {
        res.writeHead(204, {
          'Access-Control-Allow-Origin': '*',
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
            const request = JSON.parse(body) as MCPRequest;
            const response = await this.handleRequest(request);

            res.writeHead(200, {
              'Content-Type': 'application/json',
              'Access-Control-Allow-Origin': '*',
            });
            res.end(JSON.stringify(response));
          } catch (error) {
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
  private async handleRequest(request: MCPRequest): Promise<MCPResponse | null> {
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
  private handleInitialize(request: MCPRequest): MCPResponse {
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
  private handleListTools(request: MCPRequest): MCPResponse {
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
  private async handleCallTool(request: MCPRequest): Promise<MCPResponse> {
    const params = request.params as { name: string; arguments?: Record<string, unknown> };
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
    } catch (error) {
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
  private handleListResources(request: MCPRequest): MCPResponse {
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
  private async handleReadResource(request: MCPRequest): Promise<MCPResponse> {
    const params = request.params as { uri: string };
    const uri = params?.uri;

    try {
      let content: unknown;

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
    } catch (error) {
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

// Tool interface
export interface MCPTool {
  description: string;
  inputSchema: Record<string, unknown>;
  execute(args: Record<string, unknown>): Promise<unknown>;
}

// CLI entry point
async function main() {
  const args = process.argv.slice(2);
  // Precedence: built-in defaults < environment < CLI flags. Env is read here
  // (the CLI entry path) rather than in the constructor so programmatic
  // embedders are not affected by ambient environment.
  const config: Partial<MCPServerConfig> = envConfig();

  for (let i = 0; i < args.length; i++) {
    switch (args[i]) {
      case '--endpoint':
        config.endpoint = args[++i];
        break;
      case '--port':
        config.port = parseInt(args[++i], 10);
        break;
      case '--transport':
        config.transport = args[++i] as 'stdio' | 'sse';
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
export { HelixAgentTransport } from './transport';
export * from './tools';

// Run if invoked directly
if (require.main === module) {
  main().catch((error) => {
    console.error('MCP Server error:', error);
    process.exit(1);
  });
}
