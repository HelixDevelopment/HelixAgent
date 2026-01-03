"""
SuperAgent Python SDK

A Python client for the SuperAgent AI orchestration platform.
Compatible with OpenAI-style API calls.
"""

from .client import SuperAgent
from .exceptions import (
    SuperAgentError,
    AuthenticationError,
    RateLimitError,
    APIError,
    ConnectionError,
)
from .types import (
    ChatMessage,
    ChatCompletionResponse,
    ChatCompletionChoice,
    Usage,
    Model,
)

__version__ = "0.1.0"
__all__ = [
    "SuperAgent",
    "SuperAgentError",
    "AuthenticationError",
    "RateLimitError",
    "APIError",
    "ConnectionError",
    "ChatMessage",
    "ChatCompletionResponse",
    "ChatCompletionChoice",
    "Usage",
    "Model",
]
