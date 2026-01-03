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

from .types import (
    ChatMessage,
    ChatCompletionResponse,
    ChatCompletionChunk,
    Model,
    EnsembleConfig,
)
from .exceptions import (
    SuperAgentError,
    AuthenticationError,
    ConnectionError,
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
