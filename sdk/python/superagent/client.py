"""
SuperAgent Python SDK Client.

Provides OpenAI-compatible API access to the SuperAgent platform.
"""

import json
import os
from typing import (
    Any,
    Dict,
    Generator,
    List,
    Optional,
    Union,
)
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

import time as time_module

from .types import (
    ChatMessage,
    ChatCompletionResponse,
    ChatCompletionChunk,
    Model,
    EnsembleConfig,
    ParticipantConfig,
    DebateConfig,
    DebateResponse,
    DebateStatus,
    DebateResult,
    LSPPosition,
    PluginInfo,
    TemplateInfo,
)
from .exceptions import (
    SuperAgentError,
    AuthenticationError,
    ConnectionError,
    TimeoutError,
    raise_for_status,
)


class ChatCompletions:
    """Chat completions API."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def create(
        self,
        messages: List[Union[Dict[str, str], ChatMessage]],
        model: str = "superagent-ensemble",
        temperature: float = 0.7,
        max_tokens: Optional[int] = None,
        top_p: float = 1.0,
        stop: Optional[List[str]] = None,
        stream: bool = False,
        ensemble_config: Optional[EnsembleConfig] = None,
        **kwargs,
    ) -> Union[ChatCompletionResponse, Generator[ChatCompletionChunk, None, None]]:
        """
        Create a chat completion.

        Args:
            messages: List of messages in the conversation.
            model: Model to use for completion.
            temperature: Sampling temperature (0-2).
            max_tokens: Maximum tokens to generate.
            top_p: Nucleus sampling parameter.
            stop: Stop sequences.
            stream: Whether to stream the response.
            ensemble_config: Configuration for ensemble mode.
            **kwargs: Additional parameters.

        Returns:
            ChatCompletionResponse or generator of ChatCompletionChunk if streaming.

        Example:
            >>> client = SuperAgent(api_key="your-key")
            >>> response = client.chat.completions.create(
            ...     model="superagent-ensemble",
            ...     messages=[{"role": "user", "content": "Hello!"}]
            ... )
            >>> print(response.choices[0].message.content)
        """
        # Convert ChatMessage objects to dicts
        formatted_messages = []
        for msg in messages:
            if isinstance(msg, ChatMessage):
                formatted_messages.append(msg.to_dict())
            else:
                formatted_messages.append(msg)

        payload: Dict[str, Any] = {
            "model": model,
            "messages": formatted_messages,
            "temperature": temperature,
            "top_p": top_p,
            "stream": stream,
        }

        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        if stop is not None:
            payload["stop"] = stop
        if ensemble_config is not None:
            payload["ensemble_config"] = ensemble_config.to_dict()

        # Add any additional kwargs
        payload.update(kwargs)

        if stream:
            return self._stream_completion(payload)
        else:
            response = self._client._request("POST", "/v1/chat/completions", payload)
            return ChatCompletionResponse.from_dict(response)

    def _stream_completion(
        self, payload: Dict[str, Any]
    ) -> Generator[ChatCompletionChunk, None, None]:
        """Stream chat completion chunks."""
        url = f"{self._client.base_url}/v1/chat/completions"
        headers = self._client._get_headers()
        headers["Accept"] = "text/event-stream"

        request = Request(
            url,
            data=json.dumps(payload).encode("utf-8"),
            headers=headers,
            method="POST",
        )

        try:
            with urlopen(request, timeout=self._client.timeout) as response:
                buffer = ""
                for line in response:
                    line = line.decode("utf-8")
                    buffer += line

                    while "\n" in buffer:
                        line, buffer = buffer.split("\n", 1)
                        line = line.strip()

                        if not line:
                            continue
                        if line.startswith("data: "):
                            data = line[6:]
                            if data == "[DONE]":
                                return
                            try:
                                chunk_data = json.loads(data)
                                yield ChatCompletionChunk.from_dict(chunk_data)
                            except json.JSONDecodeError:
                                continue

        except HTTPError as e:
            self._client._handle_http_error(e)
        except URLError as e:
            raise ConnectionError(f"Failed to connect: {e.reason}")


class Chat:
    """Chat API namespace."""

    def __init__(self, client: "SuperAgent"):
        self.completions = ChatCompletions(client)


class Models:
    """Models API."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def list(self) -> List[Model]:
        """
        List available models.

        Returns:
            List of Model objects.

        Example:
            >>> client = SuperAgent(api_key="your-key")
            >>> models = client.models.list()
            >>> for model in models:
            ...     print(model.id)
        """
        response = self._client._request("GET", "/v1/models")
        models_data = response.get("data", response.get("models", []))
        return [Model.from_dict(m) for m in models_data]

    def retrieve(self, model_id: str) -> Model:
        """
        Retrieve a specific model.

        Args:
            model_id: The model ID to retrieve.

        Returns:
            Model object.
        """
        response = self._client._request("GET", f"/v1/models/{model_id}")
        return Model.from_dict(response)


class Debates:
    """Debates API for AI debate functionality."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def create(
        self,
        participants: List[Union[Dict[str, Any], ParticipantConfig]],
        topic: str,
        config: Optional[Union[Dict[str, Any], DebateConfig]] = None,
    ) -> DebateResponse:
        """
        Create a new debate.

        Args:
            participants: List of participant configurations.
            topic: The topic to debate.
            config: Optional debate configuration.

        Returns:
            DebateResponse with debate ID and initial status.

        Example:
            >>> participants = [
            ...     {"name": "Claude", "llm_provider": "claude"},
            ...     {"name": "GPT", "llm_provider": "openai"}
            ... ]
            >>> debate = client.debates.create(
            ...     participants=participants,
            ...     topic="Is AI beneficial for society?"
            ... )
            >>> print(debate.debate_id)
        """
        # Convert participants
        formatted_participants = []
        for p in participants:
            if isinstance(p, ParticipantConfig):
                formatted_participants.append(p.to_dict())
            else:
                formatted_participants.append(p)

        payload: Dict[str, Any] = {
            "topic": topic,
            "participants": formatted_participants,
        }

        # Merge config if provided
        if config is not None:
            if isinstance(config, DebateConfig):
                config_dict = config.to_dict()
            else:
                config_dict = config
            # Update payload with config values (except topic and participants)
            for key, value in config_dict.items():
                if key not in ("topic", "participants"):
                    payload[key] = value

        response = self._client._request("POST", "/v1/debates", payload)
        return DebateResponse.from_dict(response)

    def get(self, debate_id: str) -> DebateResponse:
        """
        Get a debate by ID.

        Args:
            debate_id: The debate ID.

        Returns:
            DebateResponse with full debate information.
        """
        response = self._client._request("GET", f"/v1/debates/{debate_id}")
        return DebateResponse.from_dict(response)

    def get_status(self, debate_id: str) -> DebateStatus:
        """
        Get the status of a debate.

        Args:
            debate_id: The debate ID.

        Returns:
            DebateStatus with current status information.
        """
        response = self._client._request("GET", f"/v1/debates/{debate_id}/status")
        return DebateStatus.from_dict(response)

    def get_results(self, debate_id: str) -> DebateResult:
        """
        Get the results of a completed debate.

        Args:
            debate_id: The debate ID.

        Returns:
            DebateResult with conclusion and details.

        Raises:
            APIError: If debate has not completed yet.
        """
        response = self._client._request("GET", f"/v1/debates/{debate_id}/results")
        return DebateResult.from_dict(response)

    def list(
        self,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        status: Optional[str] = None,
    ) -> List[DebateResponse]:
        """
        List debates.

        Args:
            limit: Maximum number of debates to return.
            offset: Number of debates to skip.
            status: Filter by status (pending, running, completed, failed).

        Returns:
            List of DebateResponse objects.
        """
        params = []
        if limit is not None:
            params.append(f"limit={limit}")
        if offset is not None:
            params.append(f"offset={offset}")
        if status is not None:
            params.append(f"status={status}")

        path = "/v1/debates"
        if params:
            path += "?" + "&".join(params)

        response = self._client._request("GET", path)
        debates_data = response.get("debates", [])
        return [DebateResponse.from_dict(d) for d in debates_data]

    def delete(self, debate_id: str) -> bool:
        """
        Delete a debate.

        Args:
            debate_id: The debate ID.

        Returns:
            True if deletion was successful.
        """
        self._client._request("DELETE", f"/v1/debates/{debate_id}")
        return True

    def wait_for_completion(
        self,
        debate_id: str,
        timeout: int = 300,
        poll_interval: float = 2.0,
    ) -> DebateResult:
        """
        Wait for a debate to complete and return results.

        Args:
            debate_id: The debate ID.
            timeout: Maximum time to wait in seconds.
            poll_interval: Time between status checks in seconds.

        Returns:
            DebateResult when debate completes.

        Raises:
            TimeoutError: If debate does not complete within timeout.
            APIError: If debate fails.
        """
        start_time = time_module.time()

        while True:
            elapsed = time_module.time() - start_time
            if elapsed >= timeout:
                raise TimeoutError(
                    f"Debate {debate_id} did not complete within {timeout} seconds"
                )

            status = self.get_status(debate_id)

            if status.status == "completed":
                return self.get_results(debate_id)
            elif status.status == "failed":
                raise SuperAgentError(
                    f"Debate {debate_id} failed: {status.error or 'Unknown error'}"
                )

            time_module.sleep(poll_interval)


class Protocols:
    """Protocols API for MCP, LSP, and ACP operations."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    # MCP Methods
    def mcp_call_tool(
        self,
        server_name: str,
        tool_name: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Call an MCP tool.

        Args:
            server_name: The MCP server name.
            tool_name: The tool to call.
            params: Optional parameters for the tool.

        Returns:
            Tool execution result.
        """
        payload = {
            "server_id": server_name,
            "tool_name": tool_name,
            "parameters": params or {},
        }
        return self._client._request("POST", "/api/v1/mcp/tools/call", payload)

    def mcp_list_tools(self, server_name: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        List available MCP tools.

        Args:
            server_name: Optional server name to filter by.

        Returns:
            List of available tools.
        """
        path = "/api/v1/mcp/tools/list"
        if server_name:
            path += f"?server_id={server_name}"
        response = self._client._request("GET", path)
        return response.get("tools", [])

    def mcp_list_servers(self) -> List[Dict[str, Any]]:
        """
        List available MCP servers.

        Returns:
            List of MCP server information.
        """
        response = self._client._request("GET", "/api/v1/mcp/servers")
        if isinstance(response, list):
            return response
        return response.get("servers", response.get("mcp_servers", []))

    # LSP Methods
    def lsp_completion(
        self,
        file_path: str,
        position: Union[Dict[str, int], LSPPosition, tuple],
    ) -> List[Dict[str, Any]]:
        """
        Get LSP code completions.

        Args:
            file_path: Path to the file.
            position: Position in file (dict with line/character, LSPPosition, or (line, char) tuple).

        Returns:
            List of completion items.
        """
        if isinstance(position, LSPPosition):
            line, character = position.line, position.character
        elif isinstance(position, tuple):
            line, character = position
        else:
            line, character = position.get("line", 0), position.get("character", 0)

        payload = {
            "file_path": file_path,
            "line": line,
            "character": character,
        }
        response = self._client._request("POST", "/api/v1/lsp/completion", payload)
        if isinstance(response, list):
            return response
        return response.get("completions", response.get("result", []))

    def lsp_hover(
        self,
        file_path: str,
        position: Union[Dict[str, int], LSPPosition, tuple],
    ) -> Dict[str, Any]:
        """
        Get LSP hover information.

        Args:
            file_path: Path to the file.
            position: Position in file.

        Returns:
            Hover information.
        """
        if isinstance(position, LSPPosition):
            line, character = position.line, position.character
        elif isinstance(position, tuple):
            line, character = position
        else:
            line, character = position.get("line", 0), position.get("character", 0)

        payload = {
            "file_path": file_path,
            "line": line,
            "character": character,
        }
        response = self._client._request("POST", "/api/v1/lsp/hover", payload)
        return response.get("result", response)

    def lsp_definition(
        self,
        file_path: str,
        position: Union[Dict[str, int], LSPPosition, tuple],
    ) -> List[Dict[str, Any]]:
        """
        Get LSP go-to-definition locations.

        Args:
            file_path: Path to the file.
            position: Position in file.

        Returns:
            List of definition locations.
        """
        if isinstance(position, LSPPosition):
            line, character = position.line, position.character
        elif isinstance(position, tuple):
            line, character = position
        else:
            line, character = position.get("line", 0), position.get("character", 0)

        payload = {
            "file_path": file_path,
            "line": line,
            "character": character,
        }
        response = self._client._request("POST", "/api/v1/lsp/definition", payload)
        if isinstance(response, list):
            return response
        return response.get("definitions", response.get("result", []))

    # ACP Methods
    def acp_execute(
        self,
        agent_name: str,
        task: Union[str, Dict[str, Any]],
        params: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Execute an ACP agent task.

        Args:
            agent_name: The agent name/ID.
            task: The task to execute (action string or task dict).
            params: Optional additional parameters.

        Returns:
            Execution result.
        """
        if isinstance(task, str):
            payload = {
                "action": task,
                "agent_id": agent_name,
                "params": params or {},
            }
        else:
            payload = {
                "agent_id": agent_name,
                **task,
            }
            if params:
                payload["params"] = {**payload.get("params", {}), **params}

        return self._client._request("POST", "/api/v1/acp/execute", payload)

    def acp_broadcast(
        self,
        message: str,
        targets: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """
        Broadcast a message to ACP agents.

        Args:
            message: The message to broadcast.
            targets: Optional list of target agent IDs.

        Returns:
            Broadcast result.
        """
        payload = {
            "message": message,
            "targets": targets or [],
        }
        return self._client._request("POST", "/api/v1/acp/broadcast", payload)

    def acp_status(self, agent_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Get ACP agent status.

        Args:
            agent_id: Optional specific agent ID.

        Returns:
            Status information.
        """
        path = "/api/v1/acp/status"
        if agent_id:
            path += f"?agent_id={agent_id}"
        return self._client._request("GET", path)


class Analytics:
    """Analytics API for metrics and health information."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def get_analytics(self, time_range: Optional[str] = None) -> Dict[str, Any]:
        """
        Get analytics metrics.

        Args:
            time_range: Optional time range (e.g., "1h", "24h", "7d").

        Returns:
            Analytics data.
        """
        path = "/api/v1/analytics/metrics"
        if time_range:
            path += f"?time_range={time_range}"
        return self._client._request("GET", path)

    def get_protocol_analytics(self, protocol: Optional[str] = None) -> Dict[str, Any]:
        """
        Get protocol-specific analytics.

        Args:
            protocol: Optional protocol name (mcp, lsp, acp).

        Returns:
            Protocol analytics data.
        """
        if protocol:
            return self._client._request("GET", f"/api/v1/analytics/metrics/{protocol}")
        return self._client._request("GET", "/api/v1/analytics/metrics")

    def get_health_status(self) -> Dict[str, Any]:
        """
        Get system health status.

        Returns:
            Health status information.
        """
        return self._client._request("GET", "/api/v1/analytics/health")


class Plugins:
    """Plugins API for plugin management."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def list(self) -> List[Dict[str, Any]]:
        """
        List all loaded plugins.

        Returns:
            List of plugin information.
        """
        response = self._client._request("GET", "/api/v1/plugins/")
        if isinstance(response, list):
            return response
        return response.get("plugins", [])

    def load(
        self,
        name: str,
        config: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Load a plugin.

        Args:
            name: Plugin name or path.
            config: Optional plugin configuration.

        Returns:
            Plugin load result.
        """
        payload: Dict[str, Any] = {"path": name}
        if config:
            payload["config"] = config
        return self._client._request("POST", "/api/v1/plugins/load", payload)

    def unload(self, name: str) -> bool:
        """
        Unload a plugin.

        Args:
            name: Plugin ID or name.

        Returns:
            True if unload was successful.
        """
        self._client._request("DELETE", f"/api/v1/plugins/{name}")
        return True

    def execute(
        self,
        name: str,
        params: Optional[Dict[str, Any]] = None,
        operation: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Execute a plugin operation.

        Args:
            name: Plugin ID or name.
            params: Parameters for the operation.
            operation: Optional operation name.

        Returns:
            Execution result.
        """
        payload: Dict[str, Any] = {"params": params or {}}
        if operation:
            payload["operation"] = operation
        return self._client._request("POST", f"/api/v1/plugins/{name}/execute", payload)


class Templates:
    """Templates API for template management and generation."""

    def __init__(self, client: "SuperAgent"):
        self._client = client

    def list(self, protocol: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        List available templates.

        Args:
            protocol: Optional protocol filter.

        Returns:
            List of template information.
        """
        path = "/api/v1/templates/"
        if protocol:
            path += f"?protocol={protocol}"
        response = self._client._request("GET", path)
        if isinstance(response, list):
            return response
        return response.get("templates", [])

    def get(self, name: str) -> Dict[str, Any]:
        """
        Get a template by name.

        Args:
            name: Template name or ID.

        Returns:
            Template information.
        """
        return self._client._request("GET", f"/api/v1/templates/{name}")

    def generate(
        self,
        name: str,
        params: Optional[Dict[str, Any]] = None,
    ) -> str:
        """
        Generate code/config from a template.

        Args:
            name: Template name or ID.
            params: Template parameters.

        Returns:
            Generated content as string.
        """
        payload = {"config": params or {}}
        response = self._client._request(
            "POST", f"/api/v1/templates/{name}/generate", payload
        )
        if isinstance(response, str):
            return response
        return response.get("content", response.get("result", str(response)))


class SuperAgent:
    """
    SuperAgent Python SDK Client.

    Provides OpenAI-compatible API access to the SuperAgent platform.

    Example:
        >>> from superagent import SuperAgent
        >>>
        >>> client = SuperAgent(
        ...     api_key="your-api-key",
        ...     base_url="http://localhost:8080"
        ... )
        >>>
        >>> response = client.chat.completions.create(
        ...     model="superagent-ensemble",
        ...     messages=[
        ...         {"role": "system", "content": "You are a helpful assistant."},
        ...         {"role": "user", "content": "Hello!"}
        ...     ]
        ... )
        >>>
        >>> print(response.choices[0].message.content)
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        base_url: str = "http://localhost:8080",
        timeout: int = 60,
        default_headers: Optional[Dict[str, str]] = None,
    ):
        """
        Initialize the SuperAgent client.

        Args:
            api_key: API key for authentication. Falls back to SUPERAGENT_API_KEY env var.
            base_url: Base URL for the SuperAgent API.
            timeout: Request timeout in seconds.
            default_headers: Additional headers to include in all requests.
        """
        self.api_key = api_key or os.environ.get("SUPERAGENT_API_KEY")
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.default_headers = default_headers or {}

        # API namespaces
        self.chat = Chat(self)
        self.models = Models(self)
        self.debates = Debates(self)
        self.protocols = Protocols(self)
        self.analytics = Analytics(self)
        self.plugins = Plugins(self)
        self.templates = Templates(self)

    def _get_headers(self) -> Dict[str, str]:
        """Get headers for API requests."""
        headers = {
            "Content-Type": "application/json",
            "User-Agent": "superagent-python/0.1.0",
        }
        headers.update(self.default_headers)

        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        return headers

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Make an API request."""
        url = f"{self.base_url}{path}"
        headers = self._get_headers()

        body = None
        if data is not None:
            body = json.dumps(data).encode("utf-8")

        request = Request(url, data=body, headers=headers, method=method)

        try:
            with urlopen(request, timeout=self.timeout) as response:
                response_data = response.read().decode("utf-8")
                return json.loads(response_data) if response_data else {}

        except HTTPError as e:
            self._handle_http_error(e)
        except URLError as e:
            raise ConnectionError(f"Failed to connect to {url}: {e.reason}")

    def _handle_http_error(self, error: HTTPError) -> None:
        """Handle HTTP errors."""
        try:
            response_data = json.loads(error.read().decode("utf-8"))
        except (json.JSONDecodeError, UnicodeDecodeError):
            response_data = {"error": error.reason}

        raise_for_status(error.code, response_data)

    def health(self) -> Dict[str, Any]:
        """
        Check API health.

        Returns:
            Health status dictionary.
        """
        return self._request("GET", "/health")

    def providers(self) -> List[Dict[str, Any]]:
        """
        List available providers.

        Returns:
            List of provider information.
        """
        response = self._request("GET", "/v1/providers")
        return response.get("providers", [])
