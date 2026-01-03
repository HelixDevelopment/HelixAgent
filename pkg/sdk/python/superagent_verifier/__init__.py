"""
SuperAgent Verifier SDK for Python

A Python SDK for interacting with the SuperAgent LLMsVerifier API.
Provides model verification, scoring, and health monitoring capabilities.
"""

from .client import VerifierClient
from .models import (
    VerificationRequest,
    VerificationResult,
    BatchVerifyRequest,
    BatchVerifyResult,
    CodeVisibilityRequest,
    CodeVisibilityResult,
    ScoreResult,
    ScoreComponents,
    ModelWithScore,
    ProviderHealth,
    ScoringWeights,
)
from .exceptions import (
    VerifierError,
    APIError,
    ValidationError,
    AuthenticationError,
    NotFoundError,
)

__version__ = "1.0.0"
__all__ = [
    "VerifierClient",
    "VerificationRequest",
    "VerificationResult",
    "BatchVerifyRequest",
    "BatchVerifyResult",
    "CodeVisibilityRequest",
    "CodeVisibilityResult",
    "ScoreResult",
    "ScoreComponents",
    "ModelWithScore",
    "ProviderHealth",
    "ScoringWeights",
    "VerifierError",
    "APIError",
    "ValidationError",
    "AuthenticationError",
    "NotFoundError",
]
