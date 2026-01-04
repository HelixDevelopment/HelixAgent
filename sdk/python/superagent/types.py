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


@dataclass
class ParticipantConfig:
    """Configuration for a debate participant."""
    name: str
    participant_id: Optional[str] = None
    role: Optional[str] = None
    llm_provider: Optional[str] = None
    llm_model: Optional[str] = None
    max_rounds: Optional[int] = None
    timeout: Optional[int] = None
    weight: float = 1.0

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API requests."""
        d = {"name": self.name}
        if self.participant_id:
            d["participant_id"] = self.participant_id
        if self.role:
            d["role"] = self.role
        if self.llm_provider:
            d["llm_provider"] = self.llm_provider
        if self.llm_model:
            d["llm_model"] = self.llm_model
        if self.max_rounds is not None:
            d["max_rounds"] = self.max_rounds
        if self.timeout is not None:
            d["timeout"] = self.timeout
        if self.weight != 1.0:
            d["weight"] = self.weight
        return d

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ParticipantConfig":
        """Create from API response dictionary."""
        return cls(
            name=data.get("name", ""),
            participant_id=data.get("participant_id"),
            role=data.get("role"),
            llm_provider=data.get("llm_provider"),
            llm_model=data.get("llm_model"),
            max_rounds=data.get("max_rounds"),
            timeout=data.get("timeout"),
            weight=data.get("weight", 1.0),
        )


@dataclass
class DebateConfig:
    """Configuration for creating a debate."""
    topic: str
    participants: List[ParticipantConfig]
    debate_id: Optional[str] = None
    max_rounds: int = 3
    timeout: int = 300
    strategy: str = "consensus"
    enable_cognee: bool = False
    metadata: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API requests."""
        d = {
            "topic": self.topic,
            "participants": [p.to_dict() for p in self.participants],
            "max_rounds": self.max_rounds,
            "timeout": self.timeout,
            "strategy": self.strategy,
            "enable_cognee": self.enable_cognee,
        }
        if self.debate_id:
            d["debate_id"] = self.debate_id
        if self.metadata:
            d["metadata"] = self.metadata
        return d


@dataclass
class DebateResponse:
    """Response from debate creation or retrieval."""
    debate_id: str
    status: str
    topic: str
    max_rounds: int = 3
    participants: int = 0
    timeout: float = 300.0
    created_at: Optional[int] = None
    start_time: Optional[int] = None
    end_time: Optional[int] = None
    duration_seconds: Optional[float] = None
    error: Optional[str] = None
    result: Optional[Dict[str, Any]] = None
    message: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DebateResponse":
        """Create from API response dictionary."""
        return cls(
            debate_id=data.get("debate_id", ""),
            status=data.get("status", ""),
            topic=data.get("topic", ""),
            max_rounds=data.get("max_rounds", 3),
            participants=data.get("participants", 0),
            timeout=data.get("timeout", 300.0),
            created_at=data.get("created_at"),
            start_time=data.get("start_time"),
            end_time=data.get("end_time"),
            duration_seconds=data.get("duration_seconds"),
            error=data.get("error"),
            result=data.get("result"),
            message=data.get("message"),
        )


@dataclass
class DebateStatus:
    """Status of a debate."""
    debate_id: str
    status: str
    start_time: int
    end_time: Optional[int] = None
    duration_seconds: Optional[float] = None
    max_rounds: Optional[int] = None
    timeout_seconds: Optional[float] = None
    error: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DebateStatus":
        """Create from API response dictionary."""
        return cls(
            debate_id=data.get("debate_id", ""),
            status=data.get("status", ""),
            start_time=data.get("start_time", 0),
            end_time=data.get("end_time"),
            duration_seconds=data.get("duration_seconds"),
            max_rounds=data.get("max_rounds"),
            timeout_seconds=data.get("timeout_seconds"),
            error=data.get("error"),
        )


@dataclass
class DebateResult:
    """Result of a completed debate."""
    debate_id: str
    conclusion: str
    consensus_reached: bool = False
    final_score: float = 0.0
    rounds: List[Dict[str, Any]] = field(default_factory=list)
    participants_summary: Dict[str, Any] = field(default_factory=dict)
    metadata: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DebateResult":
        """Create from API response dictionary."""
        return cls(
            debate_id=data.get("debate_id", data.get("DebateID", "")),
            conclusion=data.get("conclusion", data.get("Conclusion", "")),
            consensus_reached=data.get("consensus_reached", data.get("ConsensusReached", False)),
            final_score=data.get("final_score", data.get("FinalScore", 0.0)),
            rounds=data.get("rounds", data.get("Rounds", [])),
            participants_summary=data.get("participants_summary", data.get("ParticipantsSummary", {})),
            metadata=data.get("metadata", data.get("Metadata", {})),
        )


@dataclass
class LSPPosition:
    """Position in a file for LSP operations."""
    line: int
    character: int

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API requests."""
        return {
            "line": self.line,
            "character": self.character,
        }


@dataclass
class PluginInfo:
    """Information about a plugin."""
    id: str
    name: str
    version: str
    enabled: bool = True
    protocol: Optional[str] = None
    description: Optional[str] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PluginInfo":
        """Create from API response dictionary."""
        return cls(
            id=data.get("id", ""),
            name=data.get("name", ""),
            version=data.get("version", ""),
            enabled=data.get("enabled", True),
            protocol=data.get("protocol"),
            description=data.get("description"),
            metadata=data.get("metadata", {}),
        )


@dataclass
class TemplateInfo:
    """Information about a template."""
    id: str
    name: str
    description: str = ""
    protocol: str = ""
    parameters: List[Dict[str, Any]] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TemplateInfo":
        """Create from API response dictionary."""
        return cls(
            id=data.get("id", ""),
            name=data.get("name", ""),
            description=data.get("description", ""),
            protocol=data.get("protocol", ""),
            parameters=data.get("parameters", []),
            metadata=data.get("metadata", {}),
        )
