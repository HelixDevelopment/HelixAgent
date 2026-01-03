"""
Type definitions for SuperAgent SDK.
"""

from dataclasses import dataclass, field
from typing import List, Optional, Dict, Any
from datetime import datetime


@dataclass
class ChatMessage:
    """A message in a chat conversation."""
    role: str  # "system", "user", "assistant"
    content: str
    name: Optional[str] = None
    tool_calls: Optional[List[Dict[str, Any]]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API requests."""
        d = {"role": self.role, "content": self.content}
        if self.name:
            d["name"] = self.name
        if self.tool_calls:
            d["tool_calls"] = self.tool_calls
        return d

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ChatMessage":
        """Create from API response dictionary."""
        return cls(
            role=data.get("role", ""),
            content=data.get("content", ""),
            name=data.get("name"),
            tool_calls=data.get("tool_calls"),
        )


@dataclass
class Usage:
    """Token usage information."""
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Usage":
        """Create from API response dictionary."""
        return cls(
            prompt_tokens=data.get("prompt_tokens", 0),
            completion_tokens=data.get("completion_tokens", 0),
            total_tokens=data.get("total_tokens", 0),
        )


@dataclass
class ChatCompletionChoice:
    """A single completion choice."""
    index: int
    message: ChatMessage
    finish_reason: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ChatCompletionChoice":
        """Create from API response dictionary."""
        return cls(
            index=data.get("index", 0),
            message=ChatMessage.from_dict(data.get("message", {})),
            finish_reason=data.get("finish_reason"),
        )


@dataclass
class ChatCompletionResponse:
    """Response from chat completions endpoint."""
    id: str
    object: str
    created: int
    model: str
    choices: List[ChatCompletionChoice]
    usage: Optional[Usage] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ChatCompletionResponse":
        """Create from API response dictionary."""
        return cls(
            id=data.get("id", ""),
            object=data.get("object", "chat.completion"),
            created=data.get("created", 0),
            model=data.get("model", ""),
            choices=[ChatCompletionChoice.from_dict(c) for c in data.get("choices", [])],
            usage=Usage.from_dict(data["usage"]) if data.get("usage") else None,
        )


@dataclass
class StreamDelta:
    """Delta content in streaming response."""
    role: Optional[str] = None
    content: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StreamDelta":
        """Create from API response dictionary."""
        return cls(
            role=data.get("role"),
            content=data.get("content"),
        )


@dataclass
class StreamChoice:
    """A single streaming choice."""
    index: int
    delta: StreamDelta
    finish_reason: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StreamChoice":
        """Create from API response dictionary."""
        return cls(
            index=data.get("index", 0),
            delta=StreamDelta.from_dict(data.get("delta", {})),
            finish_reason=data.get("finish_reason"),
        )


@dataclass
class ChatCompletionChunk:
    """A chunk in streaming chat completion."""
    id: str
    object: str
    created: int
    model: str
    choices: List[StreamChoice]

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ChatCompletionChunk":
        """Create from API response dictionary."""
        return cls(
            id=data.get("id", ""),
            object=data.get("object", "chat.completion.chunk"),
            created=data.get("created", 0),
            model=data.get("model", ""),
            choices=[StreamChoice.from_dict(c) for c in data.get("choices", [])],
        )


@dataclass
class Model:
    """Model information."""
    id: str
    object: str = "model"
    created: int = 0
    owned_by: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Model":
        """Create from API response dictionary."""
        return cls(
            id=data.get("id", ""),
            object=data.get("object", "model"),
            created=data.get("created", 0),
            owned_by=data.get("owned_by", ""),
        )


@dataclass
class EnsembleConfig:
    """Configuration for ensemble mode."""
    strategy: str = "confidence_weighted"
    min_providers: int = 2
    confidence_threshold: float = 0.8
    fallback_to_best: bool = True
    timeout: int = 30
    preferred_providers: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API requests."""
        return {
            "strategy": self.strategy,
            "min_providers": self.min_providers,
            "confidence_threshold": self.confidence_threshold,
            "fallback_to_best": self.fallback_to_best,
            "timeout": self.timeout,
            "preferred_providers": self.preferred_providers,
        }
