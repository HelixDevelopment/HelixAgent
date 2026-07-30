#!/usr/bin/env node
/**
 * HelixAgent Generic MCP Server
 *
 * Provides MCP protocol support for Tier 2-3 CLI agents that don't have
 * rich plugin systems. Supports stdio and SSE transports.
 */
export interface MCPServerConfig {
    endpoint: string;
    transport: 'stdio' | 'sse';
    port?: number;
    preferHTTP3?: boolean;
    enableTOON?: boolean;
    enableBrotli?: boolean;
}
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
export declare function envConfig(env?: NodeJS.ProcessEnv): Partial<MCPServerConfig>;
/**
 * Generic MCP Server for HelixAgent
 */
export declare class HelixAgentMCPServer {
    private config;
    private transport;
    private tools;
    constructor(config?: Partial<MCPServerConfig>);
    /**
     * Start the MCP server
     */
    start(): Promise<void>;
    /**
     * Run in stdio mode
     */
    private runStdio;
    /**
     * Run in SSE mode
     */
    private runSSE;
    /**
     * Handle MCP request
     */
    private handleRequest;
    /**
     * Handle initialize
     */
    private handleInitialize;
    /**
     * Handle tools/list
     */
    private handleListTools;
    /**
     * Handle tools/call
     */
    private handleCallTool;
    /**
     * Handle resources/list
     */
    private handleListResources;
    /**
     * Handle resources/read
     */
    private handleReadResource;
}
export interface MCPTool {
    description: string;
    inputSchema: Record<string, unknown>;
    execute(args: Record<string, unknown>): Promise<unknown>;
}
export { HelixAgentTransport } from './transport';
export * from './tools';
//# sourceMappingURL=index.d.ts.map