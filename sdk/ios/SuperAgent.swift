//
//  SuperAgent.swift
//  SuperAgent Protocol Enhancement iOS SDK
//
//  Created by SuperAgent
//  Copyright © 2024 SuperAgent. All rights reserved.
//

import Foundation

/// Main client class for SuperAgent Protocol Enhancement API
public class SuperAgentClient {
    private let baseURL: URL
    private let session: URLSession
    private let apiKey: String?

    /// Initialize the SuperAgent client
    /// - Parameters:
    ///   - baseURL: The base URL of the SuperAgent API server
    ///   - apiKey: Optional API key for authentication
    ///   - timeout: Request timeout in seconds (default: 30)
    public init(baseURL: String = "http://localhost:8080", apiKey: String? = nil, timeout: TimeInterval = 30) {
        self.baseURL = URL(string: baseURL)!
        self.apiKey = apiKey

        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = timeout
        configuration.timeoutIntervalForResource = timeout
        self.session = URLSession(configuration: configuration)
    }

    private func makeRequest(endpoint: String, method: String = "GET", body: [String: Any]? = nil) async throws -> [String: Any] {
        let url = baseURL.appendingPathComponent(endpoint)
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        if let apiKey = apiKey {
            request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        if let body = body {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
        }

        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw SuperAgentError.invalidResponse
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let errorMessage = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw SuperAgentError.httpError(httpResponse.statusCode, errorMessage)
        }

        return try JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:]
    }

    // MARK: - MCP Protocol Methods

    /// Call an MCP tool
    public func mcpCallTool(serverId: String, toolName: String, parameters: [String: Any] = [:]) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/mcp/tools/call", method: "POST", body: [
            "server_id": serverId,
            "tool_name": toolName,
            "parameters": parameters
        ])
    }

    /// List MCP tools
    public func mcpListTools(serverId: String? = nil) async throws -> [String: Any] {
        var endpoint = "/api/v1/mcp/tools/list"
        if let serverId = serverId {
            endpoint += "?server_id=\(serverId)"
        }
        return try await makeRequest(endpoint: endpoint)
    }

    /// List MCP servers
    public func mcpListServers() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/mcp/servers")
    }

    // MARK: - LSP Protocol Methods

    /// Get code completions
    public func lspCompletion(filePath: String, line: Int, character: Int) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/lsp/completion", method: "POST", body: [
            "file_path": filePath,
            "line": line,
            "character": character
        ])
    }

    /// Get hover information
    public func lspHover(filePath: String, line: Int, character: Int) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/lsp/hover", method: "POST", body: [
            "file_path": filePath,
            "line": line,
            "character": character
        ])
    }

    /// Get definition location
    public func lspDefinition(filePath: String, line: Int, character: Int) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/lsp/definition", method: "POST", body: [
            "file_path": filePath,
            "line": line,
            "character": character
        ])
    }

    /// Get file diagnostics
    public func lspDiagnostics(filePath: String) async throws -> [String: Any] {
        let encodedPath = filePath.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? filePath
        return try await makeRequest(endpoint: "/api/v1/lsp/diagnostics?file_path=\(encodedPath)")
    }

    // MARK: - ACP Protocol Methods

    /// Execute action on agent
    public func acpExecute(action: String, agentId: String = "default", params: [String: Any] = [:]) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/acp/execute", method: "POST", body: [
            "action": action,
            "agent_id": agentId,
            "params": params
        ])
    }

    /// Broadcast message to agents
    public func acpBroadcast(message: String, targets: [String]) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/acp/broadcast", method: "POST", body: [
            "message": message,
            "targets": targets
        ])
    }

    /// Get agent status
    public func acpStatus(agentId: String? = nil) async throws -> [String: Any] {
        var endpoint = "/api/v1/acp/status"
        if let agentId = agentId {
            endpoint += "?agent_id=\(agentId)"
        }
        return try await makeRequest(endpoint: endpoint)
    }

    // MARK: - Analytics Methods

    /// Get all analytics
    public func getAnalytics() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/analytics/metrics")
    }

    /// Get protocol-specific analytics
    public func getProtocolAnalytics(protocol: String) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/analytics/metrics/\(protocol)")
    }

    /// Get system health status
    public func getHealthStatus() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/analytics/health")
    }

    /// Record a request for analytics
    public func recordRequest(protocol: String, method: String, duration: TimeInterval, success: Bool = true, errorType: String = "") async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/analytics/record", method: "POST", body: [
            "protocol": protocol,
            "method": method,
            "duration": duration,
            "success": success,
            "error_type": errorType
        ])
    }

    // MARK: - Plugin Methods

    /// List loaded plugins
    public func listPlugins() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/plugins/")
    }

    /// Load a plugin
    public func loadPlugin(path: String) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/plugins/load", method: "POST", body: [
            "path": path
        ])
    }

    /// Unload a plugin
    public func unloadPlugin(pluginId: String) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/plugins/\(pluginId)", method: "DELETE")
    }

    /// Execute plugin operation
    public func executePlugin(pluginId: String, operation: String, params: [String: Any] = [:]) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/plugins/\(pluginId)/execute", method: "POST", body: [
            "operation": operation,
            "params": params
        ])
    }

    /// Search plugin marketplace
    public func searchMarketplace(query: String = "", protocol: String = "") async throws -> [String: Any] {
        var components = URLComponents(string: "\(baseURL)/api/v1/plugins/marketplace")
        var queryItems: [URLQueryItem] = []

        if !query.isEmpty {
            queryItems.append(URLQueryItem(name: "q", value: query))
        }
        if !protocol.isEmpty {
            queryItems.append(URLQueryItem(name: "protocol", value: protocol))
        }

        components?.queryItems = queryItems

        guard let url = components?.url else {
            throw SuperAgentError.invalidURL
        }

        var request = URLRequest(url: url)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let apiKey = apiKey {
            request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse, (200...299).contains(httpResponse.statusCode) else {
            throw SuperAgentError.invalidResponse
        }

        return try JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:]
    }

    // MARK: - Template Methods

    /// List templates
    public func listTemplates(protocol: String = "") async throws -> [String: Any] {
        var endpoint = "/api/v1/templates/"
        if !protocol.isEmpty {
            endpoint += "?protocol=\(protocol)"
        }
        return try await makeRequest(endpoint: endpoint)
    }

    /// Get template details
    public func getTemplate(templateId: String) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/templates/\(templateId)")
    }

    /// Generate from template
    public func generateFromTemplate(templateId: String, config: [String: Any] = [:]) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/templates/\(templateId)/generate", method: "POST", body: [
            "config": config
        ])
    }

    // MARK: - System Methods

    /// Health check
    public func health() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/health")
    }

    /// System status
    public func status() async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/api/v1/status")
    }

    /// Get Prometheus metrics
    public func metrics() async throws -> String {
        let url = baseURL.appendingPathComponent("/api/v1/metrics")
        var request = URLRequest(url: url)
        if let apiKey = apiKey {
            request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        let (data, _) = try await session.data(for: request)
        return String(data: data, encoding: .utf8) ?? ""
    }
}

// MARK: - Error Types

public enum SuperAgentError: Error {
    case invalidResponse
    case invalidURL
    case httpError(Int, String)
}

// MARK: - Utility Classes

/// Workflow orchestrator for complex operations
public class WorkflowOrchestrator {
    private let client: SuperAgentClient

    public init(client: SuperAgentClient) {
        self.client = client
    }

    /// Execute MCP workflow
    public func executeMCPWorkflow(serverId: String, operations: [[String: Any]]) async throws -> [[String: Any]] {
        var results: [[String: Any]] = []

        for operation in operations {
            guard let tool = operation["tool"] as? String,
                  let params = operation["params"] as? [String: Any] else {
                throw SuperAgentError.invalidResponse
            }

            do {
                let result = try await client.mcpCallTool(serverId: serverId, toolName: tool, parameters: params)
                results.append([
                    "operation": operation,
                    "result": result,
                    "success": true
                ])
            } catch {
                results.append([
                    "operation": operation,
                    "error": error.localizedDescription,
                    "success": false
                ])
            }
        }

        return results
    }

    /// Execute LSP workflow
    public func executeLSPWorkflow(filePath: String, operations: [[String: Any]]) async throws -> [[String: Any]] {
        var results: [[String: Any]] = []

        for operation in operations {
            guard let type = operation["type"] as? String,
                  let line = operation["line"] as? Int,
                  let character = operation["character"] as? Int else {
                throw SuperAgentError.invalidResponse
            }

            do {
                var result: [String: Any]
                switch type {
                case "completion":
                    result = try await client.lspCompletion(filePath: filePath, line: line, character: character)
                case "hover":
                    result = try await client.lspHover(filePath: filePath, line: line, character: character)
                case "definition":
                    result = try await client.lspDefinition(filePath: filePath, line: line, character: character)
                default:
                    throw SuperAgentError.invalidResponse
                }

                results.append([
                    "operation": operation,
                    "result": result,
                    "success": true
                ])
            } catch {
                results.append([
                    "operation": operation,
                    "error": error.localizedDescription,
                    "success": false
                ])
            }
        }

        return results
    }

    /// Execute ACP workflow
    public func executeACPWorkflow(agentId: String, operations: [[String: Any]]) async throws -> [[String: Any]] {
        var results: [[String: Any]] = []

        for operation in operations {
            do {
                var result: [String: Any]
                if let action = operation["action"] as? String {
                    let params = operation["params"] as? [String: Any] ?? [:]
                    result = try await client.acpExecute(action: action, agentId: agentId, params: params)
                } else if let message = operation["message"] as? String,
                          let targets = operation["targets"] as? [String] {
                    result = try await client.acpBroadcast(message: message, targets: targets)
                } else {
                    throw SuperAgentError.invalidResponse
                }

                results.append([
                    "operation": operation,
                    "result": result,
                    "success": true
                ])
            } catch {
                results.append([
                    "operation": operation,
                    "error": error.localizedDescription,
                    "success": false
                ])
            }
        }

        return results
    }
}

/// Analytics monitor for real-time metrics
public class AnalyticsMonitor {
    private let client: SuperAgentClient
    private var timer: Timer?
    private let interval: TimeInterval

    public init(client: SuperAgentClient, interval: TimeInterval = 30.0) {
        self.client = client
        self.interval = interval
    }

    /// Start monitoring
    public func start() {
        timer = Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { [weak self] _ in
            Task {
                await self?.performMonitoring()
            }
        }
    }

    /// Stop monitoring
    public func stop() {
        timer?.invalidate()
        timer = nil
    }

    private func performMonitoring() async {
        do {
            let health = try await client.getHealthStatus()
            let metrics = try await client.getAnalytics()

            // Post notification for monitoring updates
            NotificationCenter.default.post(
                name: NSNotification.Name("SuperAgentMetricsUpdate"),
                object: nil,
                userInfo: [
                    "health": health,
                    "metrics": metrics,
                    "timestamp": Date()
                ]
            )
        } catch {
            print("Monitoring error: \(error.localizedDescription)")
        }
    }

    /// Get comprehensive report
    public func getReport() async throws -> [String: Any] {
        async let analyticsTask = client.getAnalytics()
        async let healthTask = client.getHealthStatus()

        let analytics = try await analyticsTask
        let health = try await healthTask

        return [
            "timestamp": Date().ISO8601Format(),
            "analytics": analytics,
            "health": health,
            "summary": [
                "total_requests": analytics["summary"]?["total_requests"] ?? 0,
                "error_rate": analytics["summary"]?["error_rate"] ?? 0,
                "system_health": health["overall_status"] ?? "unknown"
            ]
        ]
    }
}

// MARK: - Extensions

extension SuperAgentClient {
    /// Create client from environment variables
    public static func fromEnvironment() -> SuperAgentClient {
        let baseURL = ProcessInfo.processInfo.environment["SUPERAGENT_URL"] ?? "http://localhost:8080"
        let apiKey = ProcessInfo.processInfo.environment["SUPERAGENT_API_KEY"]
        return SuperAgentClient(baseURL: baseURL, apiKey: apiKey)
    }

    /// Initialize development environment
    public func initializeDevelopmentEnvironment() async throws -> [String: Any] {
        // Generate default MCP integration
        let mcpTemplate = try await generateFromTemplate(templateId: "mcp-basic-integration", config: [
            "enabled": true,
            "timeout": "30s"
        ])

        // Generate default LSP integration
        let lspTemplate = try await generateFromTemplate(templateId: "lsp-code-completion", config: [
            "language": "swift",
            "enabled": true
        ])

        return [
            "mcp_template": mcpTemplate,
            "lsp_template": lspTemplate,
            "message": "Development environment initialized"
        ]
    }
}