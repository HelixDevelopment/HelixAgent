package com.superagent.protocol

import kotlinx.coroutines.*
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.json.JSONArray
import java.io.IOException
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