package com.superagent.protocol

import kotlinx.coroutines.*
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flow
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.json.JSONArray
import java.io.BufferedReader
import java.io.IOException
import java.io.InputStreamReader
import java.util.concurrent.TimeUnit

/**
 * SuperAgent Protocol Enhancement Android SDK
 * Kotlin client library for SuperAgent Protocol Enhancement API
 *
 * @version 1.0.0
 * @author SuperAgent
 */
class SuperAgentClient(
    private val baseUrl: String = "http://localhost:8080",
    private val apiKey: String? = null,
    private val timeoutSeconds: Long = 30
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(timeoutSeconds, TimeUnit.SECONDS)
        .readTimeout(timeoutSeconds, TimeUnit.SECONDS)
        .writeTimeout(timeoutSeconds, TimeUnit.SECONDS)
        .build()

    private val jsonMediaType = "application/json; charset=utf-8".toMediaType()

    private suspend fun makeRequest(
        endpoint: String,
        method: String = "GET",
        body: JSONObject? = null
    ): JSONObject = suspendCancellableCoroutine { continuation ->
        val url = if (endpoint.startsWith("/")) {
            "$baseUrl$endpoint"
        } else {
            "$baseUrl/api/v1/$endpoint"
        }

        val requestBuilder = Request.Builder()
            .url(url)
            .addHeader("Content-Type", "application/json")

        if (apiKey != null) {
            requestBuilder.addHeader("Authorization", "Bearer $apiKey")
        }

        val requestBody = body?.toString()?.toRequestBody(jsonMediaType)
        val request = when (method) {
            "POST" -> requestBuilder.post(requestBody ?: "".toRequestBody(jsonMediaType))
            "PUT" -> requestBuilder.put(requestBody ?: "".toRequestBody(jsonMediaType))
            "DELETE" -> requestBuilder.delete(requestBody)
            else -> requestBuilder.get()
        }.build()

        client.newCall(request).enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                if (continuation.isActive) {
                    continuation.resumeWithException(e)
                }
            }

            override fun onResponse(call: Call, response: Response) {
                if (!continuation.isActive) return

                try {
                    val responseBody = response.body?.string()
                    if (response.isSuccessful && responseBody != null) {
                        val json = JSONObject(responseBody)
                        continuation.resume(json)
                    } else {
                        val errorMsg = responseBody ?: "Unknown error"
                        continuation.resumeWithException(
                            SuperAgentException(response.code, errorMsg)
                        )
                    }
                } catch (e: Exception) {
                    continuation.resumeWithException(e)
                } finally {
                    response.close()
                }
            }
        })

        continuation.invokeOnCancellation {
            // Cancel the HTTP call if coroutine is cancelled
        }
    }

    // MCP Protocol Methods
    suspend fun mcpCallTool(serverId: String, toolName: String, parameters: JSONObject = JSONObject()): JSONObject {
        val body = JSONObject().apply {
            put("server_id", serverId)
            put("tool_name", toolName)
            put("parameters", parameters)
        }
        return makeRequest("/api/v1/mcp/tools/call", "POST", body)
    }

    suspend fun mcpListTools(serverId: String? = null): JSONObject {
        val endpoint = if (serverId != null) {
            "/api/v1/mcp/tools/list?server_id=$serverId"
        } else {
            "/api/v1/mcp/tools/list"
        }
        return makeRequest(endpoint)
    }

    suspend fun mcpListServers(): JSONObject {
        return makeRequest("/api/v1/mcp/servers")
    }

    // ==================== Chat Completions API ====================

    /**
     * Create a chat completion
     * @param model The model to use (e.g., "superagent-ensemble")
     * @param messages List of chat messages
     * @param temperature Sampling temperature (0.0 to 2.0)
     * @param maxTokens Maximum tokens to generate
     * @param topP Top-p sampling parameter
     * @param stop Stop sequences
     * @return Chat completion response
     */
    suspend fun chatCompletion(
        model: String,
        messages: List<ChatMessage>,
        temperature: Double = 0.7,
        maxTokens: Int = 1000,
        topP: Double = 1.0,
        stop: List<String>? = null
    ): ChatCompletionResponse {
        val messagesArray = JSONArray()
        messages.forEach { msg ->
            messagesArray.put(JSONObject().apply {
                put("role", msg.role)
                put("content", msg.content)
                msg.name?.let { put("name", it) }
            })
        }

        val body = JSONObject().apply {
            put("model", model)
            put("messages", messagesArray)
            put("temperature", temperature)
            put("max_tokens", maxTokens)
            put("top_p", topP)
            stop?.let {
                val stopArray = JSONArray()
                it.forEach { s -> stopArray.put(s) }
                put("stop", stopArray)
            }
        }

        val response = makeRequest("/v1/chat/completions", "POST", body)
        return ChatCompletionResponse.fromJson(response)
    }

    /**
     * Create a chat completion with ensemble configuration
     */
    suspend fun chatCompletionWithEnsemble(
        model: String,
        messages: List<ChatMessage>,
        ensembleConfig: EnsembleConfig,
        temperature: Double = 0.7,
        maxTokens: Int = 1000
    ): ChatCompletionResponse {
        val messagesArray = JSONArray()
        messages.forEach { msg ->
            messagesArray.put(JSONObject().apply {
                put("role", msg.role)
                put("content", msg.content)
            })
        }

        val body = JSONObject().apply {
            put("model", model)
            put("messages", messagesArray)
            put("temperature", temperature)
            put("max_tokens", maxTokens)
            put("ensemble_config", ensembleConfig.toJson())
        }

        val response = makeRequest("/v1/chat/completions", "POST", body)
        return ChatCompletionResponse.fromJson(response)
    }

    // ==================== Streaming Chat Completions API ====================

    /**
     * Create a streaming chat completion
     * @param model The model to use (e.g., "superagent-ensemble")
     * @param messages List of chat messages
     * @param temperature Sampling temperature (0.0 to 2.0)
     * @param maxTokens Maximum tokens to generate
     * @param topP Top-p sampling parameter
     * @param stop Stop sequences
     * @return Flow of ChatCompletionChunk objects
     */
    fun chatCompletionStream(
        model: String,
        messages: List<ChatMessage>,
        temperature: Double = 0.7,
        maxTokens: Int = 1000,
        topP: Double = 1.0,
        stop: List<String>? = null
    ): Flow<ChatCompletionChunk> = callbackFlow {
        val messagesArray = JSONArray()
        messages.forEach { msg ->
            messagesArray.put(JSONObject().apply {
                put("role", msg.role)
                put("content", msg.content)
                msg.name?.let { put("name", it) }
            })
        }

        val body = JSONObject().apply {
            put("model", model)
            put("messages", messagesArray)
            put("temperature", temperature)
            put("max_tokens", maxTokens)
            put("top_p", topP)
            put("stream", true)
            stop?.let {
                val stopArray = JSONArray()
                it.forEach { s -> stopArray.put(s) }
                put("stop", stopArray)
            }
        }

        val requestBody = body.toString().toRequestBody(jsonMediaType)
        val request = Request.Builder()
            .url("$baseUrl/v1/chat/completions")
            .addHeader("Content-Type", "application/json")
            .addHeader("Accept", "text/event-stream")
            .apply { apiKey?.let { addHeader("Authorization", "Bearer $it") } }
            .post(requestBody)
            .build()

        // Use a separate client with longer timeout for streaming
        val streamingClient = client.newBuilder()
            .readTimeout(5, TimeUnit.MINUTES)
            .build()

        val call = streamingClient.newCall(request)

        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                close(e)
            }

            override fun onResponse(call: Call, response: Response) {
                if (!response.isSuccessful) {
                    close(SuperAgentException(response.code, response.message))
                    return
                }

                val responseBody = response.body
                if (responseBody == null) {
                    close(SuperAgentException(500, "Empty response body"))
                    return
                }

                try {
                    val reader = BufferedReader(InputStreamReader(responseBody.byteStream()))
                    var buffer = StringBuilder()

                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        val currentLine = line ?: continue

                        // Handle SSE format
                        if (currentLine.startsWith("data:")) {
                            val dataContent = currentLine.substring(5).trim()

                            // Check for stream end
                            if (dataContent == "[DONE]") {
                                break
                            }

                            // Parse JSON chunk
                            if (dataContent.isNotEmpty()) {
                                try {
                                    val json = JSONObject(dataContent)
                                    val chunk = ChatCompletionChunk.fromJson(json)
                                    trySend(chunk)
                                } catch (e: Exception) {
                                    // Log parsing error but continue processing
                                    println("SSE parsing warning: ${e.message}")
                                }
                            }
                        } else if (currentLine.isEmpty()) {
                            // Empty line marks end of SSE event, reset buffer
                            buffer = StringBuilder()
                        } else {
                            // Accumulate partial data
                            buffer.append(currentLine)
                        }
                    }

                    reader.close()
                    close()
                } catch (e: Exception) {
                    close(e)
                } finally {
                    response.close()
                }
            }
        })

        awaitClose {
            call.cancel()
        }
    }

    /**
     * Create a streaming chat completion with ensemble configuration
     */
    fun chatCompletionStreamWithEnsemble(
        model: String,
        messages: List<ChatMessage>,
        ensembleConfig: EnsembleConfig,
        temperature: Double = 0.7,
        maxTokens: Int = 1000
    ): Flow<ChatCompletionChunk> = callbackFlow {
        val messagesArray = JSONArray()
        messages.forEach { msg ->
            messagesArray.put(JSONObject().apply {
                put("role", msg.role)
                put("content", msg.content)
                msg.name?.let { put("name", it) }
            })
        }

        val body = JSONObject().apply {
            put("model", model)
            put("messages", messagesArray)
            put("temperature", temperature)
            put("max_tokens", maxTokens)
            put("stream", true)
            put("ensemble_config", ensembleConfig.toJson())
        }

        val requestBody = body.toString().toRequestBody(jsonMediaType)
        val request = Request.Builder()
            .url("$baseUrl/v1/chat/completions")
            .addHeader("Content-Type", "application/json")
            .addHeader("Accept", "text/event-stream")
            .apply { apiKey?.let { addHeader("Authorization", "Bearer $it") } }
            .post(requestBody)
            .build()

        // Use a separate client with longer timeout for streaming
        val streamingClient = client.newBuilder()
            .readTimeout(5, TimeUnit.MINUTES)
            .build()

        val call = streamingClient.newCall(request)

        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                close(e)
            }

            override fun onResponse(call: Call, response: Response) {
                if (!response.isSuccessful) {
                    close(SuperAgentException(response.code, response.message))
                    return
                }

                val responseBody = response.body
                if (responseBody == null) {
                    close(SuperAgentException(500, "Empty response body"))
                    return
                }

                try {
                    val reader = BufferedReader(InputStreamReader(responseBody.byteStream()))

                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        val currentLine = line ?: continue

                        if (currentLine.startsWith("data:")) {
                            val dataContent = currentLine.substring(5).trim()

                            if (dataContent == "[DONE]") {
                                break
                            }

                            if (dataContent.isNotEmpty()) {
                                try {
                                    val json = JSONObject(dataContent)
                                    val chunk = ChatCompletionChunk.fromJson(json)
                                    trySend(chunk)
                                } catch (e: Exception) {
                                    println("SSE parsing warning: ${e.message}")
                                }
                            }
                        }
                    }

                    reader.close()
                    close()
                } catch (e: Exception) {
                    close(e)
                } finally {
                    response.close()
                }
            }
        })

        awaitClose {
            call.cancel()
        }
    }

    // ==================== AI Debate API ====================

    /**
     * Create a new debate
     * @param topic The debate topic
     * @param participants List of debate participants
     * @param maxRounds Maximum number of debate rounds
     * @param timeout Timeout in seconds
     * @param strategy Debate strategy (e.g., "consensus", "adversarial")
     * @return Debate creation response
     */
    suspend fun createDebate(
        topic: String,
        participants: List<DebateParticipant>,
        maxRounds: Int = 3,
        timeout: Int = 300,
        strategy: String = "consensus"
    ): DebateResponse {
        val participantsArray = JSONArray()
        participants.forEach { p ->
            participantsArray.put(JSONObject().apply {
                put("name", p.name)
                p.role?.let { put("role", it) }
                p.llmProvider?.let { put("llm_provider", it) }
                p.llmModel?.let { put("llm_model", it) }
                p.weight?.let { put("weight", it) }
            })
        }

        val body = JSONObject().apply {
            put("topic", topic)
            put("participants", participantsArray)
            put("max_rounds", maxRounds)
            put("timeout", timeout)
            put("strategy", strategy)
        }

        val response = makeRequest("/v1/debates", "POST", body)
        return DebateResponse.fromJson(response)
    }

    /**
     * Get debate by ID
     */
    suspend fun getDebate(debateId: String): DebateResponse {
        val response = makeRequest("/v1/debates/$debateId")
        return DebateResponse.fromJson(response)
    }

    /**
     * Get debate status
     */
    suspend fun getDebateStatus(debateId: String): DebateStatus {
        val response = makeRequest("/v1/debates/$debateId/status")
        return DebateStatus.fromJson(response)
    }

    /**
     * Get debate results (when completed)
     */
    suspend fun getDebateResults(debateId: String): DebateResult {
        val response = makeRequest("/v1/debates/$debateId/results")
        return DebateResult.fromJson(response)
    }

    /**
     * List all debates
     * @param status Optional status filter (pending, running, completed, failed)
     */
    suspend fun listDebates(status: String? = null): List<DebateResponse> {
        val endpoint = if (status != null) "/v1/debates?status=$status" else "/v1/debates"
        val response = makeRequest(endpoint)
        val debates = mutableListOf<DebateResponse>()
        val debatesArray = response.optJSONArray("debates") ?: JSONArray()
        for (i in 0 until debatesArray.length()) {
            debates.add(DebateResponse.fromJson(debatesArray.getJSONObject(i)))
        }
        return debates
    }

    /**
     * Delete a debate
     */
    suspend fun deleteDebate(debateId: String): JSONObject {
        return makeRequest("/v1/debates/$debateId", "DELETE")
    }

    /**
     * Wait for debate completion with polling
     */
    suspend fun waitForDebateCompletion(
        debateId: String,
        pollIntervalMs: Long = 5000,
        timeoutMs: Long = 600000
    ): DebateResult {
        val startTime = System.currentTimeMillis()
        while (System.currentTimeMillis() - startTime < timeoutMs) {
            val status = getDebateStatus(debateId)
            when (status.status) {
                "completed" -> return getDebateResults(debateId)
                "failed" -> throw SuperAgentException(500, "Debate failed: ${status.error}")
            }
            delay(pollIntervalMs)
        }
        throw SuperAgentException(408, "Debate did not complete within timeout")
    }

    // LSP Protocol Methods
    suspend fun lspCompletion(filePath: String, line: Int, character: Int): JSONObject {
        val body = JSONObject().apply {
            put("file_path", filePath)
            put("line", line)
            put("character", character)
        }
        return makeRequest("/api/v1/lsp/completion", "POST", body)
    }

    suspend fun lspHover(filePath: String, line: Int, character: Int): JSONObject {
        val body = JSONObject().apply {
            put("file_path", filePath)
            put("line", line)
            put("character", character)
        }
        return makeRequest("/api/v1/lsp/hover", "POST", body)
    }

    suspend fun lspDefinition(filePath: String, line: Int, character: Int): JSONObject {
        val body = JSONObject().apply {
            put("file_path", filePath)
            put("line", line)
            put("character", character)
        }
        return makeRequest("/api/v1/lsp/definition", "POST", body)
    }

    suspend fun lspDiagnostics(filePath: String): JSONObject {
        val encodedPath = java.net.URLEncoder.encode(filePath, "UTF-8")
        return makeRequest("/api/v1/lsp/diagnostics?file_path=$encodedPath")
    }

    // ACP Protocol Methods
    suspend fun acpExecute(action: String, agentId: String = "default", params: JSONObject = JSONObject()): JSONObject {
        val body = JSONObject().apply {
            put("action", action)
            put("agent_id", agentId)
            put("params", params)
        }
        return makeRequest("/api/v1/acp/execute", "POST", body)
    }

    suspend fun acpBroadcast(message: String, targets: JSONArray): JSONObject {
        val body = JSONObject().apply {
            put("message", message)
            put("targets", targets)
        }
        return makeRequest("/api/v1/acp/broadcast", "POST", body)
    }

    suspend fun acpStatus(agentId: String? = null): JSONObject {
        val endpoint = if (agentId != null) {
            "/api/v1/acp/status?agent_id=$agentId"
        } else {
            "/api/v1/acp/status"
        }
        return makeRequest(endpoint)
    }

    // Analytics Methods
    suspend fun getAnalytics(): JSONObject {
        return makeRequest("/api/v1/analytics/metrics")
    }

    suspend fun getProtocolAnalytics(protocol: String): JSONObject {
        return makeRequest("/api/v1/analytics/metrics/$protocol")
    }

    suspend fun getHealthStatus(): JSONObject {
        return makeRequest("/api/v1/analytics/health")
    }

    suspend fun recordRequest(protocol: String, method: String, duration: Long, success: Boolean = true, errorType: String = ""): JSONObject {
        val body = JSONObject().apply {
            put("protocol", protocol)
            put("method", method)
            put("duration", duration)
            put("success", success)
            put("error_type", errorType)
        }
        return makeRequest("/api/v1/analytics/record", "POST", body)
    }

    // Plugin Methods
    suspend fun listPlugins(): JSONObject {
        return makeRequest("/api/v1/plugins/")
    }

    suspend fun loadPlugin(path: String): JSONObject {
        val body = JSONObject().apply {
            put("path", path)
        }
        return makeRequest("/api/v1/plugins/load", "POST", body)
    }

    suspend fun unloadPlugin(pluginId: String): JSONObject {
        return makeRequest("/api/v1/plugins/$pluginId", "DELETE")
    }

    suspend fun executePlugin(pluginId: String, operation: String, params: JSONObject = JSONObject()): JSONObject {
        val body = JSONObject().apply {
            put("operation", operation)
            put("params", params)
        }
        return makeRequest("/api/v1/plugins/$pluginId/execute", "POST", body)
    }

    suspend fun searchMarketplace(query: String = "", protocol: String = ""): JSONObject {
        val params = mutableListOf<String>()
        if (query.isNotEmpty()) params.add("q=$query")
        if (protocol.isNotEmpty()) params.add("protocol=$protocol")

        val queryString = if (params.isNotEmpty()) "?${params.joinToString("&")}" else ""
        return makeRequest("/api/v1/plugins/marketplace$queryString")
    }

    suspend fun registerPluginInMarketplace(plugin: JSONObject): JSONObject {
        return makeRequest("/api/v1/plugins/marketplace/register", "POST", plugin)
    }

    // Template Methods
    suspend fun listTemplates(protocol: String = ""): JSONObject {
        val endpoint = if (protocol.isNotEmpty()) {
            "/api/v1/templates/?protocol=$protocol"
        } else {
            "/api/v1/templates/"
        }
        return makeRequest(endpoint)
    }

    suspend fun getTemplate(templateId: String): JSONObject {
        return makeRequest("/api/v1/templates/$templateId")
    }

    suspend fun generateFromTemplate(templateId: String, config: JSONObject = JSONObject()): JSONObject {
        val body = JSONObject().apply {
            put("config", config)
        }
        return makeRequest("/api/v1/templates/$templateId/generate", "POST", body)
    }

    // System Methods
    suspend fun health(): JSONObject {
        return makeRequest("/api/v1/health")
    }

    suspend fun status(): JSONObject {
        return makeRequest("/api/v1/status")
    }

    suspend fun metrics(): String {
        val request = Request.Builder()
            .url("$baseUrl/api/v1/metrics")
            .apply { if (apiKey != null) addHeader("Authorization", "Bearer $apiKey") }
            .build()

        return suspendCancellableCoroutine { continuation ->
            client.newCall(request).enqueue(object : Callback {
                override fun onFailure(call: Call, e: IOException) {
                    if (continuation.isActive) {
                        continuation.resumeWithException(e)
                    }
                }

                override fun onResponse(call: Call, response: Response) {
                    if (!continuation.isActive) return

                    try {
                        val responseBody = response.body?.string() ?: ""
                        continuation.resume(responseBody)
                    } catch (e: Exception) {
                        continuation.resumeWithException(e)
                    } finally {
                        response.close()
                    }
                }
            })
        }
    }

    companion object {
        /**
         * Create client from environment variables
         */
        fun fromEnvironment(): SuperAgentClient {
            val baseUrl = System.getenv("SUPERAGENT_URL") ?: "http://localhost:8080"
            val apiKey = System.getenv("SUPERAGENT_API_KEY")
            return SuperAgentClient(baseUrl, apiKey)
        }

        /**
         * Initialize development environment
         */
        suspend fun initializeDevelopmentEnvironment(client: SuperAgentClient): JSONObject {
            // Generate default MCP integration
            val mcpConfig = JSONObject().apply {
                put("enabled", true)
                put("timeout", "30s")
            }
            val mcpTemplate = client.generateFromTemplate("mcp-basic-integration", mcpConfig)

            // Generate default LSP integration
            val lspConfig = JSONObject().apply {
                put("language", "kotlin")
                put("enabled", true)
            }
            val lspTemplate = client.generateFromTemplate("lsp-code-completion", lspConfig)

            return JSONObject().apply {
                put("mcp_template", mcpTemplate)
                put("lsp_template", lspTemplate)
                put("message", "Development environment initialized")
            }
        }
    }
}

// Workflow Orchestrator
class WorkflowOrchestrator(private val client: SuperAgentClient) {

    suspend fun executeMCPWorkflow(serverId: String, operations: JSONArray): JSONArray {
        val results = JSONArray()

        for (i in 0 until operations.length()) {
            val operation = operations.getJSONObject(i)
            val tool = operation.getString("tool")
            val params = operation.optJSONObject("params") ?: JSONObject()

            try {
                val result = client.mcpCallTool(serverId, tool, params)
                val resultObj = JSONObject().apply {
                    put("operation", operation)
                    put("result", result)
                    put("success", true)
                }
                results.put(resultObj)
            } catch (e: Exception) {
                val errorObj = JSONObject().apply {
                    put("operation", operation)
                    put("error", e.message)
                    put("success", false)
                }
                results.put(errorObj)
            }
        }

        return results
    }

    suspend fun executeLSPWorkflow(filePath: String, operations: JSONArray): JSONArray {
        val results = JSONArray()

        for (i in 0 until operations.length()) {
            val operation = operations.getJSONObject(i)
            val type = operation.getString("type")
            val line = operation.getInt("line")
            val character = operation.getInt("character")

            try {
                val result = when (type) {
                    "completion" -> client.lspCompletion(filePath, line, character)
                    "hover" -> client.lspHover(filePath, line, character)
                    "definition" -> client.lspDefinition(filePath, line, character)
                    else -> throw IllegalArgumentException("Unknown LSP operation: $type")
                }

                val resultObj = JSONObject().apply {
                    put("operation", operation)
                    put("result", result)
                    put("success", true)
                }
                results.put(resultObj)
            } catch (e: Exception) {
                val errorObj = JSONObject().apply {
                    put("operation", operation)
                    put("error", e.message)
                    put("success", false)
                }
                results.put(errorObj)
            }
        }

        return results
    }

    suspend fun executeACPWorkflow(agentId: String, operations: JSONArray): JSONArray {
        val results = JSONArray()

        for (i in 0 until operations.length()) {
            val operation = operations.getJSONObject(i)

            try {
                val result = if (operation.has("action")) {
                    val action = operation.getString("action")
                    val params = operation.optJSONObject("params") ?: JSONObject()
                    client.acpExecute(action, agentId, params)
                } else if (operation.has("message")) {
                    val message = operation.getString("message")
                    val targets = operation.getJSONArray("targets")
                    client.acpBroadcast(message, targets)
                } else {
                    throw IllegalArgumentException("Invalid ACP operation")
                }

                val resultObj = JSONObject().apply {
                    put("operation", operation)
                    put("result", result)
                    put("success", true)
                }
                results.put(resultObj)
            } catch (e: Exception) {
                val errorObj = JSONObject().apply {
                    put("operation", operation)
                    put("error", e.message)
                    put("success", false)
                }
                results.put(errorObj)
            }
        }

        return results
    }
}

// Analytics Monitor
class AnalyticsMonitor(
    private val client: SuperAgentClient,
    private val intervalMs: Long = 30000
) {
    private var job: Job? = null

    fun start() {
        job = CoroutineScope(Dispatchers.IO).launch {
            while (isActive) {
                try {
                    performMonitoring()
                } catch (e: Exception) {
                    println("Monitoring error: ${e.message}")
                }
                delay(intervalMs)
            }
        }
    }

    fun stop() {
        job?.cancel()
        job = null
    }

    private suspend fun performMonitoring() {
        try {
            val health = client.getHealthStatus()
            val metrics = client.getAnalytics()

            // Emit monitoring event (you would implement your own event system)
            onMetricsUpdate(health, metrics)
        } catch (e: Exception) {
            println("Monitoring error: ${e.message}")
        }
    }

    suspend fun getReport(): JSONObject {
        val analytics = async { client.getAnalytics() }
        val health = async { client.getHealthStatus() }

        val analyticsResult = analytics.await()
        val healthResult = health.await()

        return JSONObject().apply {
            put("timestamp", java.time.Instant.now().toString())
            put("analytics", analyticsResult)
            put("health", healthResult)
            put("summary", JSONObject().apply {
                put("total_requests", analyticsResult.optJSONObject("summary")?.optLong("total_requests") ?: 0)
                put("error_rate", analyticsResult.optJSONObject("summary")?.optDouble("error_rate") ?: 0.0)
                put("system_health", healthResult.optString("overall_status", "unknown"))
            })
        }
    }

    // Override this method to handle metrics updates
    protected open fun onMetricsUpdate(health: JSONObject, metrics: JSONObject) {
        println("System health: ${health.optString("overall_status", "unknown")}")
        println("Active protocols: ${metrics.optInt("total_protocols", 0)}")
    }
}

// Exception class
class SuperAgentException(val statusCode: Int, message: String) : Exception(message)

// ==================== Data Classes ====================

// Chat Completion Types
data class ChatMessage(
    val role: String,
    val content: String,
    val name: String? = null
)

data class ChatChoice(
    val index: Int,
    val message: ChatMessage,
    val finishReason: String?
)

data class ChatUsage(
    val promptTokens: Int,
    val completionTokens: Int,
    val totalTokens: Int
)

data class ChatCompletionResponse(
    val id: String,
    val model: String,
    val created: Long,
    val choices: List<ChatChoice>,
    val usage: ChatUsage?
) {
    companion object {
        fun fromJson(json: JSONObject): ChatCompletionResponse {
            val choicesArray = json.getJSONArray("choices")
            val choices = mutableListOf<ChatChoice>()
            for (i in 0 until choicesArray.length()) {
                val choiceJson = choicesArray.getJSONObject(i)
                val messageJson = choiceJson.getJSONObject("message")
                choices.add(ChatChoice(
                    index = choiceJson.getInt("index"),
                    message = ChatMessage(
                        role = messageJson.getString("role"),
                        content = messageJson.optString("content", "")
                    ),
                    finishReason = choiceJson.optString("finish_reason")
                ))
            }

            val usageJson = json.optJSONObject("usage")
            val usage = usageJson?.let {
                ChatUsage(
                    promptTokens = it.optInt("prompt_tokens", 0),
                    completionTokens = it.optInt("completion_tokens", 0),
                    totalTokens = it.optInt("total_tokens", 0)
                )
            }

            return ChatCompletionResponse(
                id = json.getString("id"),
                model = json.getString("model"),
                created = json.getLong("created"),
                choices = choices,
                usage = usage
            )
        }
    }
}

// Streaming Chat Completion Types

/**
 * Delta content for streaming responses
 */
data class ChatDelta(
    val role: String? = null,
    val content: String? = null
) {
    companion object {
        fun fromJson(json: JSONObject): ChatDelta = ChatDelta(
            role = json.optString("role").takeIf { it.isNotEmpty() },
            content = json.optString("content").takeIf { it.isNotEmpty() }
        )
    }
}

/**
 * Streaming choice for chat completion chunks
 */
data class ChatChunkChoice(
    val index: Int,
    val delta: ChatDelta,
    val finishReason: String? = null
) {
    companion object {
        fun fromJson(json: JSONObject): ChatChunkChoice = ChatChunkChoice(
            index = json.optInt("index", 0),
            delta = ChatDelta.fromJson(json.optJSONObject("delta") ?: JSONObject()),
            finishReason = json.optString("finish_reason").takeIf { it.isNotEmpty() && it != "null" }
        )
    }
}

/**
 * Chat completion chunk for streaming responses
 */
data class ChatCompletionChunk(
    val id: String,
    val model: String,
    val created: Long,
    val choices: List<ChatChunkChoice>
) {
    /**
     * Get the delta content of the first choice
     */
    val deltaContent: String?
        get() = choices.firstOrNull()?.delta?.content

    /**
     * Check if this is the final chunk
     */
    val isFinished: Boolean
        get() = choices.firstOrNull()?.finishReason != null

    companion object {
        fun fromJson(json: JSONObject): ChatCompletionChunk {
            val choicesArray = json.optJSONArray("choices") ?: JSONArray()
            val choices = mutableListOf<ChatChunkChoice>()
            for (i in 0 until choicesArray.length()) {
                choices.add(ChatChunkChoice.fromJson(choicesArray.getJSONObject(i)))
            }

            return ChatCompletionChunk(
                id = json.optString("id", ""),
                model = json.optString("model", ""),
                created = json.optLong("created", System.currentTimeMillis() / 1000),
                choices = choices
            )
        }
    }
}

/**
 * Stream accumulator to collect chunks into a complete response
 */
class StreamAccumulator {
    private val chunks = mutableListOf<ChatCompletionChunk>()
    private val accumulatedContent = StringBuilder()
    private var id: String = ""
    private var model: String = ""
    private var created: Long = 0

    /**
     * Add a chunk to the accumulator
     */
    fun addChunk(chunk: ChatCompletionChunk) {
        chunks.add(chunk)
        if (id.isEmpty()) id = chunk.id
        if (model.isEmpty()) model = chunk.model
        if (created == 0L) created = chunk.created
        chunk.deltaContent?.let { accumulatedContent.append(it) }
    }

    /**
     * Get the accumulated content so far
     */
    val content: String
        get() = accumulatedContent.toString()

    /**
     * Get the final complete message
     */
    fun buildMessage(): ChatMessage = ChatMessage(
        role = "assistant",
        content = accumulatedContent.toString()
    )

    /**
     * Get a complete response from accumulated chunks
     */
    fun buildResponse(): ChatCompletionResponse {
        val finishReason = chunks.lastOrNull()?.choices?.firstOrNull()?.finishReason ?: "stop"

        return ChatCompletionResponse(
            id = id,
            model = model,
            created = created,
            choices = listOf(
                ChatChoice(
                    index = 0,
                    message = ChatMessage(
                        role = "assistant",
                        content = accumulatedContent.toString()
                    ),
                    finishReason = finishReason
                )
            ),
            usage = null
        )
    }

    /**
     * Reset the accumulator
     */
    fun reset() {
        chunks.clear()
        accumulatedContent.clear()
        id = ""
        model = ""
        created = 0
    }
}

data class EnsembleConfig(
    val strategy: String = "confidence_weighted",
    val minProviders: Int = 2,
    val confidenceThreshold: Double = 0.8,
    val fallbackToBest: Boolean = true,
    val preferredProviders: List<String> = emptyList()
) {
    fun toJson(): JSONObject = JSONObject().apply {
        put("strategy", strategy)
        put("min_providers", minProviders)
        put("confidence_threshold", confidenceThreshold)
        put("fallback_to_best", fallbackToBest)
        val providersArray = JSONArray()
        preferredProviders.forEach { providersArray.put(it) }
        put("preferred_providers", providersArray)
    }
}

// AI Debate Types
data class DebateParticipant(
    val name: String,
    val role: String? = null,
    val llmProvider: String? = null,
    val llmModel: String? = null,
    val weight: Double? = null
)

data class DebateResponse(
    val debateId: String,
    val status: String,
    val topic: String,
    val maxRounds: Int,
    val participants: Int,
    val createdAt: Long
) {
    companion object {
        fun fromJson(json: JSONObject): DebateResponse = DebateResponse(
            debateId = json.getString("debate_id"),
            status = json.getString("status"),
            topic = json.optString("topic", ""),
            maxRounds = json.optInt("max_rounds", 3),
            participants = json.optInt("participants", 0),
            createdAt = json.optLong("created_at", 0)
        )
    }
}

data class DebateStatus(
    val debateId: String,
    val status: String,
    val startTime: Long,
    val endTime: Long?,
    val durationSeconds: Double?,
    val error: String?
) {
    companion object {
        fun fromJson(json: JSONObject): DebateStatus = DebateStatus(
            debateId = json.getString("debate_id"),
            status = json.getString("status"),
            startTime = json.getLong("start_time"),
            endTime = if (json.has("end_time")) json.getLong("end_time") else null,
            durationSeconds = if (json.has("duration_seconds")) json.getDouble("duration_seconds") else null,
            error = json.optString("error").takeIf { it.isNotEmpty() }
        )
    }
}

data class ConsensusResult(
    val reached: Boolean,
    val confidence: Double,
    val finalPosition: String,
    val keyPoints: List<String>,
    val disagreements: List<String>
) {
    companion object {
        fun fromJson(json: JSONObject): ConsensusResult {
            val keyPoints = mutableListOf<String>()
            val keyPointsArray = json.optJSONArray("key_points") ?: JSONArray()
            for (i in 0 until keyPointsArray.length()) {
                keyPoints.add(keyPointsArray.getString(i))
            }

            val disagreements = mutableListOf<String>()
            val disagreementsArray = json.optJSONArray("disagreements") ?: JSONArray()
            for (i in 0 until disagreementsArray.length()) {
                disagreements.add(disagreementsArray.getString(i))
            }

            return ConsensusResult(
                reached = json.optBoolean("reached", false),
                confidence = json.optDouble("confidence", 0.0),
                finalPosition = json.optString("final_position", ""),
                keyPoints = keyPoints,
                disagreements = disagreements
            )
        }
    }
}

data class DebateResult(
    val debateId: String,
    val topic: String,
    val totalRounds: Int,
    val success: Boolean,
    val qualityScore: Double,
    val consensus: ConsensusResult?
) {
    companion object {
        fun fromJson(json: JSONObject): DebateResult = DebateResult(
            debateId = json.getString("debate_id"),
            topic = json.optString("topic", ""),
            totalRounds = json.optInt("total_rounds", 0),
            success = json.optBoolean("success", false),
            qualityScore = json.optDouble("quality_score", 0.0),
            consensus = json.optJSONObject("consensus")?.let { ConsensusResult.fromJson(it) }
        )
    }
}