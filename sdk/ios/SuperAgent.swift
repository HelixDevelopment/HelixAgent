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

    // MARK: - Chat Completions API

    /// Create a chat completion
    /// - Parameters:
    ///   - model: The model to use (e.g., "superagent-ensemble")
    ///   - messages: List of chat messages
    ///   - temperature: Sampling temperature (0.0 to 2.0)
    ///   - maxTokens: Maximum tokens to generate
    ///   - topP: Top-p sampling parameter
    ///   - stop: Stop sequences
    /// - Returns: Chat completion response
    public func chatCompletion(
        model: String,
        messages: [ChatMessage],
        temperature: Double = 0.7,
        maxTokens: Int = 1000,
        topP: Double = 1.0,
        stop: [String]? = nil
    ) async throws -> ChatCompletionResponse {
        var body: [String: Any] = [
            "model": model,
            "messages": messages.map { $0.toDictionary() },
            "temperature": temperature,
            "max_tokens": maxTokens,
            "top_p": topP
        ]

        if let stop = stop {
            body["stop"] = stop
        }

        let response = try await makeRequest(endpoint: "/v1/chat/completions", method: "POST", body: body)
        return ChatCompletionResponse(from: response)
    }

    /// Create a chat completion with ensemble configuration
    public func chatCompletionWithEnsemble(
        model: String,
        messages: [ChatMessage],
        ensembleConfig: EnsembleConfig,
        temperature: Double = 0.7,
        maxTokens: Int = 1000
    ) async throws -> ChatCompletionResponse {
        let body: [String: Any] = [
            "model": model,
            "messages": messages.map { $0.toDictionary() },
            "temperature": temperature,
            "max_tokens": maxTokens,
            "ensemble_config": ensembleConfig.toDictionary()
        ]

        let response = try await makeRequest(endpoint: "/v1/chat/completions", method: "POST", body: body)
        return ChatCompletionResponse(from: response)
    }

    // MARK: - Streaming Chat Completions

    /// Create a streaming chat completion
    /// - Parameters:
    ///   - model: The model to use (e.g., "superagent-ensemble")
    ///   - messages: List of chat messages
    ///   - temperature: Sampling temperature (0.0 to 2.0)
    ///   - maxTokens: Maximum tokens to generate
    ///   - topP: Top-p sampling parameter
    ///   - stop: Stop sequences
    /// - Returns: AsyncThrowingStream of ChatCompletionChunk objects
    public func chatCompletionStream(
        model: String,
        messages: [ChatMessage],
        temperature: Double = 0.7,
        maxTokens: Int = 1000,
        topP: Double = 1.0,
        stop: [String]? = nil
    ) -> AsyncThrowingStream<ChatCompletionChunk, Error> {
        return AsyncThrowingStream { continuation in
            Task {
                do {
                    try await self.performStreamingRequest(
                        model: model,
                        messages: messages,
                        temperature: temperature,
                        maxTokens: maxTokens,
                        topP: topP,
                        stop: stop,
                        ensembleConfig: nil,
                        continuation: continuation
                    )
                } catch {
                    continuation.finish(throwing: error)
                }
            }
        }
    }

    /// Create a streaming chat completion with ensemble configuration
    public func chatCompletionStreamWithEnsemble(
        model: String,
        messages: [ChatMessage],
        ensembleConfig: EnsembleConfig,
        temperature: Double = 0.7,
        maxTokens: Int = 1000
    ) -> AsyncThrowingStream<ChatCompletionChunk, Error> {
        return AsyncThrowingStream { continuation in
            Task {
                do {
                    try await self.performStreamingRequest(
                        model: model,
                        messages: messages,
                        temperature: temperature,
                        maxTokens: maxTokens,
                        topP: 1.0,
                        stop: nil,
                        ensembleConfig: ensembleConfig,
                        continuation: continuation
                    )
                } catch {
                    continuation.finish(throwing: error)
                }
            }
        }
    }

    private func performStreamingRequest(
        model: String,
        messages: [ChatMessage],
        temperature: Double,
        maxTokens: Int,
        topP: Double,
        stop: [String]?,
        ensembleConfig: EnsembleConfig?,
        continuation: AsyncThrowingStream<ChatCompletionChunk, Error>.Continuation
    ) async throws {
        let url = baseURL.appendingPathComponent("/v1/chat/completions")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")

        if let apiKey = apiKey {
            request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        var body: [String: Any] = [
            "model": model,
            "messages": messages.map { $0.toDictionary() },
            "temperature": temperature,
            "max_tokens": maxTokens,
            "top_p": topP,
            "stream": true
        ]

        if let stop = stop {
            body["stop"] = stop
        }

        if let ensembleConfig = ensembleConfig {
            body["ensemble_config"] = ensembleConfig.toDictionary()
        }

        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (bytes, response) = try await session.bytes(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw SuperAgentError.invalidResponse
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            throw SuperAgentError.httpError(httpResponse.statusCode, "Stream request failed")
        }

        var buffer = ""

        for try await byte in bytes {
            buffer.append(Character(UnicodeScalar(byte)))

            // Process complete SSE messages (data: {...}\n\n)
            while let range = buffer.range(of: "\n\n") {
                let message = String(buffer[..<range.lowerBound])
                buffer = String(buffer[range.upperBound...])

                // Parse SSE lines
                for line in message.components(separatedBy: "\n") {
                    let trimmedLine = line.trimmingCharacters(in: .whitespaces)

                    if trimmedLine.hasPrefix("data:") {
                        let dataContent = String(trimmedLine.dropFirst(5)).trimmingCharacters(in: .whitespaces)

                        // Check for stream end
                        if dataContent == "[DONE]" {
                            continuation.finish()
                            return
                        }

                        // Parse JSON chunk
                        if let jsonData = dataContent.data(using: .utf8) {
                            do {
                                if let json = try JSONSerialization.jsonObject(with: jsonData) as? [String: Any] {
                                    let chunk = ChatCompletionChunk(from: json)
                                    continuation.yield(chunk)
                                }
                            } catch {
                                // Log parsing error but continue processing
                                print("SSE parsing warning: \(error.localizedDescription)")
                            }
                        }
                    }
                }
            }
        }

        // Process any remaining buffer content
        if !buffer.isEmpty {
            for line in buffer.components(separatedBy: "\n") {
                let trimmedLine = line.trimmingCharacters(in: .whitespaces)
                if trimmedLine.hasPrefix("data:") {
                    let dataContent = String(trimmedLine.dropFirst(5)).trimmingCharacters(in: .whitespaces)
                    if dataContent != "[DONE]", let jsonData = dataContent.data(using: .utf8) {
                        if let json = try? JSONSerialization.jsonObject(with: jsonData) as? [String: Any] {
                            let chunk = ChatCompletionChunk(from: json)
                            continuation.yield(chunk)
                        }
                    }
                }
            }
        }

        continuation.finish()
    }

    // MARK: - AI Debate API

    /// Create a new debate
    /// - Parameters:
    ///   - topic: The debate topic
    ///   - participants: List of debate participants
    ///   - maxRounds: Maximum number of debate rounds
    ///   - timeout: Timeout in seconds
    ///   - strategy: Debate strategy (e.g., "consensus", "adversarial")
    /// - Returns: Debate creation response
    public func createDebate(
        topic: String,
        participants: [DebateParticipant],
        maxRounds: Int = 3,
        timeout: Int = 300,
        strategy: String = "consensus"
    ) async throws -> DebateResponse {
        let body: [String: Any] = [
            "topic": topic,
            "participants": participants.map { $0.toDictionary() },
            "max_rounds": maxRounds,
            "timeout": timeout,
            "strategy": strategy
        ]

        let response = try await makeRequest(endpoint: "/v1/debates", method: "POST", body: body)
        return DebateResponse(from: response)
    }

    /// Get debate by ID
    public func getDebate(debateId: String) async throws -> DebateResponse {
        let response = try await makeRequest(endpoint: "/v1/debates/\(debateId)")
        return DebateResponse(from: response)
    }

    /// Get debate status
    public func getDebateStatus(debateId: String) async throws -> DebateStatus {
        let response = try await makeRequest(endpoint: "/v1/debates/\(debateId)/status")
        return DebateStatus(from: response)
    }

    /// Get debate results (when completed)
    public func getDebateResults(debateId: String) async throws -> DebateResult {
        let response = try await makeRequest(endpoint: "/v1/debates/\(debateId)/results")
        return DebateResult(from: response)
    }

    /// List all debates
    /// - Parameter status: Optional status filter (pending, running, completed, failed)
    public func listDebates(status: String? = nil) async throws -> [DebateResponse] {
        var endpoint = "/v1/debates"
        if let status = status {
            endpoint += "?status=\(status)"
        }
        let response = try await makeRequest(endpoint: endpoint)
        let debatesArray = response["debates"] as? [[String: Any]] ?? []
        return debatesArray.map { DebateResponse(from: $0) }
    }

    /// Delete a debate
    public func deleteDebate(debateId: String) async throws -> [String: Any] {
        return try await makeRequest(endpoint: "/v1/debates/\(debateId)", method: "DELETE")
    }

    /// Wait for debate completion with polling
    public func waitForDebateCompletion(
        debateId: String,
        pollInterval: TimeInterval = 5.0,
        timeout: TimeInterval = 600.0
    ) async throws -> DebateResult {
        let startTime = Date()
        while Date().timeIntervalSince(startTime) < timeout {
            let status = try await getDebateStatus(debateId: debateId)
            switch status.status {
            case "completed":
                return try await getDebateResults(debateId: debateId)
            case "failed":
                throw SuperAgentError.debateFailed(status.error ?? "Unknown error")
            default:
                try await Task.sleep(nanoseconds: UInt64(pollInterval * 1_000_000_000))
            }
        }
        throw SuperAgentError.timeout
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
    case debateFailed(String)
    case timeout
    case streamDisconnected(String)
    case streamParsingError(String)
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

// MARK: - Chat Completions Data Structures

/// Chat message for completions API
public struct ChatMessage {
    public let role: String
    public let content: String
    public let name: String?

    public init(role: String, content: String, name: String? = nil) {
        self.role = role
        self.content = content
        self.name = name
    }

    func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "role": role,
            "content": content
        ]
        if let name = name {
            dict["name"] = name
        }
        return dict
    }
}

/// Chat completion choice
public struct ChatChoice {
    public let index: Int
    public let message: ChatMessage
    public let finishReason: String?

    public init(from dict: [String: Any]) {
        self.index = dict["index"] as? Int ?? 0
        let messageDict = dict["message"] as? [String: Any] ?? [:]
        self.message = ChatMessage(
            role: messageDict["role"] as? String ?? "assistant",
            content: messageDict["content"] as? String ?? "",
            name: messageDict["name"] as? String
        )
        self.finishReason = dict["finish_reason"] as? String
    }
}

/// Chat completion usage statistics
public struct ChatUsage {
    public let promptTokens: Int
    public let completionTokens: Int
    public let totalTokens: Int

    public init(from dict: [String: Any]) {
        self.promptTokens = dict["prompt_tokens"] as? Int ?? 0
        self.completionTokens = dict["completion_tokens"] as? Int ?? 0
        self.totalTokens = dict["total_tokens"] as? Int ?? 0
    }
}

/// Chat completion response
public struct ChatCompletionResponse {
    public let id: String
    public let object: String
    public let created: Int
    public let model: String
    public let choices: [ChatChoice]
    public let usage: ChatUsage?

    public init(from dict: [String: Any]) {
        self.id = dict["id"] as? String ?? ""
        self.object = dict["object"] as? String ?? "chat.completion"
        self.created = dict["created"] as? Int ?? Int(Date().timeIntervalSince1970)
        self.model = dict["model"] as? String ?? ""
        let choicesArray = dict["choices"] as? [[String: Any]] ?? []
        self.choices = choicesArray.map { ChatChoice(from: $0) }
        if let usageDict = dict["usage"] as? [String: Any] {
            self.usage = ChatUsage(from: usageDict)
        } else {
            self.usage = nil
        }
    }

    /// Get the content of the first choice
    public var content: String {
        choices.first?.message.content ?? ""
    }
}

// MARK: - Streaming Chat Completion Data Structures

/// Delta content for streaming responses
public struct ChatDelta {
    public let role: String?
    public let content: String?

    public init(from dict: [String: Any]) {
        self.role = dict["role"] as? String
        self.content = dict["content"] as? String
    }
}

/// Streaming choice for chat completion chunks
public struct ChatChunkChoice {
    public let index: Int
    public let delta: ChatDelta
    public let finishReason: String?

    public init(from dict: [String: Any]) {
        self.index = dict["index"] as? Int ?? 0
        let deltaDict = dict["delta"] as? [String: Any] ?? [:]
        self.delta = ChatDelta(from: deltaDict)
        self.finishReason = dict["finish_reason"] as? String
    }
}

/// Chat completion chunk for streaming responses
public struct ChatCompletionChunk {
    public let id: String
    public let object: String
    public let created: Int
    public let model: String
    public let choices: [ChatChunkChoice]

    public init(from dict: [String: Any]) {
        self.id = dict["id"] as? String ?? ""
        self.object = dict["object"] as? String ?? "chat.completion.chunk"
        self.created = dict["created"] as? Int ?? Int(Date().timeIntervalSince1970)
        self.model = dict["model"] as? String ?? ""
        let choicesArray = dict["choices"] as? [[String: Any]] ?? []
        self.choices = choicesArray.map { ChatChunkChoice(from: $0) }
    }

    /// Get the delta content of the first choice
    public var deltaContent: String? {
        choices.first?.delta.content
    }

    /// Check if this is the final chunk
    public var isFinished: Bool {
        choices.first?.finishReason != nil
    }
}

/// Stream accumulator to collect chunks into a complete response
public class StreamAccumulator {
    private var chunks: [ChatCompletionChunk] = []
    private var accumulatedContent: String = ""
    private var id: String = ""
    private var model: String = ""
    private var created: Int = 0

    public init() {}

    /// Add a chunk to the accumulator
    public func addChunk(_ chunk: ChatCompletionChunk) {
        chunks.append(chunk)
        if id.isEmpty { id = chunk.id }
        if model.isEmpty { model = chunk.model }
        if created == 0 { created = chunk.created }
        if let content = chunk.deltaContent {
            accumulatedContent += content
        }
    }

    /// Get the accumulated content so far
    public var content: String {
        return accumulatedContent
    }

    /// Get the final complete message
    public func buildMessage() -> ChatMessage {
        return ChatMessage(role: "assistant", content: accumulatedContent)
    }

    /// Get a complete response from accumulated chunks
    public func buildResponse() -> ChatCompletionResponse {
        let choice: [String: Any] = [
            "index": 0,
            "message": [
                "role": "assistant",
                "content": accumulatedContent
            ],
            "finish_reason": chunks.last?.choices.first?.finishReason ?? "stop"
        ]

        let dict: [String: Any] = [
            "id": id,
            "object": "chat.completion",
            "created": created,
            "model": model,
            "choices": [choice]
        ]

        return ChatCompletionResponse(from: dict)
    }

    /// Reset the accumulator
    public func reset() {
        chunks.removeAll()
        accumulatedContent = ""
        id = ""
        model = ""
        created = 0
    }
}

/// Ensemble configuration for multi-provider requests
public struct EnsembleConfig {
    public let strategy: String
    public let minProviders: Int
    public let confidenceThreshold: Double
    public let fallbackToBest: Bool
    public let providers: [String]?

    public init(
        strategy: String = "confidence_weighted",
        minProviders: Int = 2,
        confidenceThreshold: Double = 0.7,
        fallbackToBest: Bool = true,
        providers: [String]? = nil
    ) {
        self.strategy = strategy
        self.minProviders = minProviders
        self.confidenceThreshold = confidenceThreshold
        self.fallbackToBest = fallbackToBest
        self.providers = providers
    }

    func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "strategy": strategy,
            "min_providers": minProviders,
            "confidence_threshold": confidenceThreshold,
            "fallback_to_best": fallbackToBest
        ]
        if let providers = providers {
            dict["providers"] = providers
        }
        return dict
    }
}

// MARK: - AI Debate Data Structures

/// Debate participant configuration
public struct DebateParticipant {
    public let name: String
    public let role: String
    public let provider: String
    public let model: String
    public let systemPrompt: String?

    public init(
        name: String,
        role: String,
        provider: String,
        model: String,
        systemPrompt: String? = nil
    ) {
        self.name = name
        self.role = role
        self.provider = provider
        self.model = model
        self.systemPrompt = systemPrompt
    }

    func toDictionary() -> [String: Any] {
        var dict: [String: Any] = [
            "name": name,
            "role": role,
            "provider": provider,
            "model": model
        ]
        if let systemPrompt = systemPrompt {
            dict["system_prompt"] = systemPrompt
        }
        return dict
    }
}

/// Debate creation/retrieval response
public struct DebateResponse {
    public let id: String
    public let topic: String
    public let status: String
    public let participants: [String]
    public let maxRounds: Int
    public let createdAt: String

    public init(from dict: [String: Any]) {
        self.id = dict["id"] as? String ?? dict["debate_id"] as? String ?? ""
        self.topic = dict["topic"] as? String ?? ""
        self.status = dict["status"] as? String ?? "pending"
        self.participants = dict["participants"] as? [String] ?? []
        self.maxRounds = dict["max_rounds"] as? Int ?? 3
        self.createdAt = dict["created_at"] as? String ?? ""
    }
}

/// Debate status information
public struct DebateStatus {
    public let id: String
    public let status: String
    public let currentRound: Int
    public let totalRounds: Int
    public let progress: Double
    public let error: String?

    public init(from dict: [String: Any]) {
        self.id = dict["id"] as? String ?? dict["debate_id"] as? String ?? ""
        self.status = dict["status"] as? String ?? "unknown"
        self.currentRound = dict["current_round"] as? Int ?? 0
        self.totalRounds = dict["total_rounds"] as? Int ?? 0
        self.progress = dict["progress"] as? Double ?? 0.0
        self.error = dict["error"] as? String
    }
}

/// Consensus result from debate
public struct ConsensusResult {
    public let reached: Bool
    public let confidence: Double
    public let finalPosition: String
    public let supportingArguments: [String]

    public init(from dict: [String: Any]) {
        self.reached = dict["reached"] as? Bool ?? false
        self.confidence = dict["confidence"] as? Double ?? 0.0
        self.finalPosition = dict["final_position"] as? String ?? ""
        self.supportingArguments = dict["supporting_arguments"] as? [String] ?? []
    }
}

/// Complete debate result
public struct DebateResult {
    public let id: String
    public let topic: String
    public let status: String
    public let rounds: [[String: Any]]
    public let consensus: ConsensusResult?
    public let duration: Double
    public let completedAt: String

    public init(from dict: [String: Any]) {
        self.id = dict["id"] as? String ?? dict["debate_id"] as? String ?? ""
        self.topic = dict["topic"] as? String ?? ""
        self.status = dict["status"] as? String ?? "completed"
        self.rounds = dict["rounds"] as? [[String: Any]] ?? []
        if let consensusDict = dict["consensus"] as? [String: Any] {
            self.consensus = ConsensusResult(from: consensusDict)
        } else {
            self.consensus = nil
        }
        self.duration = dict["duration"] as? Double ?? 0.0
        self.completedAt = dict["completed_at"] as? String ?? ""
    }

    /// Check if consensus was reached
    public var consensusReached: Bool {
        consensus?.reached ?? false
    }

    /// Get the final position if consensus was reached
    public var finalPosition: String? {
        consensus?.finalPosition
    }
}