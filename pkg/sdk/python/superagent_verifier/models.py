"""
Data models for the SuperAgent Verifier SDK.
"""

from dataclasses import dataclass, field
from typing import Dict, List, Optional
from datetime import datetime


@dataclass
class VerificationRequest:
    """Request for verifying a model."""
    model_id: str
    provider: str
    tests: Optional[List[str]] = None
    timeout: Optional[int] = None
    retry_count: Optional[int] = None

    def to_dict(self) -> dict:
        d = {"model_id": self.model_id, "provider": self.provider}
        if self.tests:
            d["tests"] = self.tests
        if self.timeout:
            d["timeout"] = self.timeout
        if self.retry_count:
            d["retry_count"] = self.retry_count
        return d


@dataclass
class VerificationResult:
    """Result of a model verification."""
    model_id: str
    provider: str
    verified: bool
    score: float
    overall_score: float
    score_suffix: str
    code_visible: bool
    tests: Dict[str, bool]
    verification_time_ms: int
    message: Optional[str] = None

    @classmethod
    def from_dict(cls, data: dict) -> "VerificationResult":
        return cls(
            model_id=data["model_id"],
            provider=data["provider"],
            verified=data["verified"],
            score=data["score"],
            overall_score=data["overall_score"],
            score_suffix=data["score_suffix"],
            code_visible=data["code_visible"],
            tests=data.get("tests", {}),
            verification_time_ms=data.get("verification_time_ms", 0),
            message=data.get("message"),
        )


@dataclass
class BatchVerifyRequest:
    """Request for batch verifying models."""
    models: List[Dict[str, str]]

    def to_dict(self) -> dict:
        return {"models": self.models}


@dataclass
class BatchVerifySummary:
    """Summary of batch verification."""
    total: int
    verified: int
    failed: int


@dataclass
class BatchVerifyResult:
    """Result of batch model verification."""
    results: List[VerificationResult]
    summary: BatchVerifySummary

    @classmethod
    def from_dict(cls, data: dict) -> "BatchVerifyResult":
        results = [VerificationResult.from_dict(r) for r in data["results"]]
        summary = BatchVerifySummary(
            total=data["summary"]["total"],
            verified=data["summary"]["verified"],
            failed=data["summary"]["failed"],
        )
        return cls(results=results, summary=summary)


@dataclass
class CodeVisibilityRequest:
    """Request for testing code visibility."""
    model_id: str
    provider: str
    language: Optional[str] = "python"

    def to_dict(self) -> dict:
        return {
            "model_id": self.model_id,
            "provider": self.provider,
            "language": self.language,
        }


@dataclass
class CodeVisibilityResult:
    """Result of code visibility test."""
    model_id: str
    provider: str
    code_visible: bool
    language: str
    prompt: str
    response: str
    confidence: float

    @classmethod
    def from_dict(cls, data: dict) -> "CodeVisibilityResult":
        return cls(
            model_id=data["model_id"],
            provider=data["provider"],
            code_visible=data["code_visible"],
            language=data["language"],
            prompt=data["prompt"],
            response=data["response"],
            confidence=data["confidence"],
        )


@dataclass
class ScoreComponents:
    """Score components breakdown."""
    speed_score: float
    efficiency_score: float
    cost_score: float
    capability_score: float
    recency_score: float

    @classmethod
    def from_dict(cls, data: dict) -> "ScoreComponents":
        return cls(
            speed_score=data["speed_score"],
            efficiency_score=data["efficiency_score"],
            cost_score=data["cost_score"],
            capability_score=data["capability_score"],
            recency_score=data["recency_score"],
        )


@dataclass
class ScoreResult:
    """Model score result."""
    model_id: str
    model_name: str
    overall_score: float
    score_suffix: str
    components: ScoreComponents
    calculated_at: str
    data_source: str

    @classmethod
    def from_dict(cls, data: dict) -> "ScoreResult":
        return cls(
            model_id=data["model_id"],
            model_name=data["model_name"],
            overall_score=data["overall_score"],
            score_suffix=data["score_suffix"],
            components=ScoreComponents.from_dict(data["components"]),
            calculated_at=data["calculated_at"],
            data_source=data["data_source"],
        )


@dataclass
class ModelWithScore:
    """Model with its score information."""
    model_id: str
    name: str
    provider: str
    overall_score: float
    score_suffix: str
    rank: int

    @classmethod
    def from_dict(cls, data: dict) -> "ModelWithScore":
        return cls(
            model_id=data["model_id"],
            name=data["name"],
            provider=data["provider"],
            overall_score=data["overall_score"],
            score_suffix=data["score_suffix"],
            rank=data.get("rank", 0),
        )


@dataclass
class ProviderHealth:
    """Provider health status."""
    provider_id: str
    provider_name: str
    healthy: bool
    circuit_state: str
    failure_count: int
    success_count: int
    avg_response_ms: int
    uptime_percent: float
    last_checked_at: str
    last_success_at: Optional[str] = None
    last_failure_at: Optional[str] = None

    @classmethod
    def from_dict(cls, data: dict) -> "ProviderHealth":
        return cls(
            provider_id=data["provider_id"],
            provider_name=data["provider_name"],
            healthy=data["healthy"],
            circuit_state=data["circuit_state"],
            failure_count=data["failure_count"],
            success_count=data["success_count"],
            avg_response_ms=data["avg_response_ms"],
            uptime_percent=data["uptime_percent"],
            last_checked_at=data["last_checked_at"],
            last_success_at=data.get("last_success_at"),
            last_failure_at=data.get("last_failure_at"),
        )


@dataclass
class ScoringWeights:
    """Scoring weights configuration."""
    response_speed: float
    model_efficiency: float
    cost_effectiveness: float
    capability: float
    recency: float

    def to_dict(self) -> dict:
        return {
            "response_speed": self.response_speed,
            "model_efficiency": self.model_efficiency,
            "cost_effectiveness": self.cost_effectiveness,
            "capability": self.capability,
            "recency": self.recency,
        }

    @classmethod
    def from_dict(cls, data: dict) -> "ScoringWeights":
        return cls(
            response_speed=data["response_speed"],
            model_efficiency=data["model_efficiency"],
            cost_effectiveness=data["cost_effectiveness"],
            capability=data["capability"],
            recency=data["recency"],
        )

    @classmethod
    def default(cls) -> "ScoringWeights":
        return cls(
            response_speed=0.25,
            model_efficiency=0.20,
            cost_effectiveness=0.25,
            capability=0.20,
            recency=0.10,
        )

    def validate(self) -> bool:
        total = (
            self.response_speed
            + self.model_efficiency
            + self.cost_effectiveness
            + self.capability
            + self.recency
        )
        return 0.99 <= total <= 1.01
