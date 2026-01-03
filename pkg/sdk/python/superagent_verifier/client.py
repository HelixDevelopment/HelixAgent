"""
Main client for the SuperAgent Verifier SDK.
"""

import requests
from typing import List, Optional, Dict, Any
from urllib.parse import urljoin

from .models import (
    VerificationRequest,
    VerificationResult,
    BatchVerifyRequest,
    BatchVerifyResult,
    CodeVisibilityRequest,
    CodeVisibilityResult,
    ScoreResult,
    ModelWithScore,
    ProviderHealth,
    ScoringWeights,
)
from .exceptions import (
    APIError,
    AuthenticationError,
    NotFoundError,
    ValidationError,
)


class VerifierClient:
    """
    Client for interacting with the SuperAgent LLMsVerifier API.

    Example usage:
        >>> client = VerifierClient(base_url="http://localhost:8081", api_key="your-api-key")
        >>> result = client.verify_model("gpt-4", "openai")
        >>> print(f"Verified: {result.verified}, Score: {result.overall_score}")
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8081",
        api_key: Optional[str] = None,
        timeout: int = 30,
    ):
        """
        Initialize the verifier client.

        Args:
            base_url: The base URL of the verifier API
            api_key: Optional API key for authentication
            timeout: Request timeout in seconds
        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self.session = requests.Session()

        if api_key:
            self.session.headers["Authorization"] = f"Bearer {api_key}"
        self.session.headers["Content-Type"] = "application/json"

    def _request(
        self,
        method: str,
        path: str,
        json: Optional[dict] = None,
        params: Optional[dict] = None,
    ) -> dict:
        """Make an HTTP request to the API."""
        url = urljoin(self.base_url, path)

        try:
            response = self.session.request(
                method=method,
                url=url,
                json=json,
                params=params,
                timeout=self.timeout,
            )
        except requests.Timeout:
            raise APIError("Request timed out", status_code=408)
        except requests.RequestException as e:
            raise APIError(f"Request failed: {str(e)}")

        if response.status_code == 401:
            raise AuthenticationError("Authentication failed")
        elif response.status_code == 404:
            raise NotFoundError(f"Resource not found: {path}")
        elif response.status_code == 400:
            raise ValidationError(response.json().get("error", "Validation error"))
        elif response.status_code >= 400:
            raise APIError(
                response.json().get("error", "API error"),
                status_code=response.status_code,
                response=response.json() if response.text else None,
            )

        return response.json()

    # Verification methods

    def verify_model(
        self,
        model_id: str,
        provider: str,
        tests: Optional[List[str]] = None,
    ) -> VerificationResult:
        """
        Verify a specific model.

        Args:
            model_id: The model ID to verify
            provider: The provider name
            tests: Optional list of specific tests to run

        Returns:
            VerificationResult with verification details
        """
        req = VerificationRequest(model_id=model_id, provider=provider, tests=tests)
        data = self._request("POST", "/api/v1/verifier/verify", json=req.to_dict())
        return VerificationResult.from_dict(data)

    def batch_verify(
        self,
        models: List[Dict[str, str]],
    ) -> BatchVerifyResult:
        """
        Verify multiple models in batch.

        Args:
            models: List of dicts with "model_id" and "provider" keys

        Returns:
            BatchVerifyResult with results and summary
        """
        req = BatchVerifyRequest(models=models)
        data = self._request("POST", "/api/v1/verifier/verify/batch", json=req.to_dict())
        return BatchVerifyResult.from_dict(data)

    def get_verification_status(self, model_id: str) -> VerificationResult:
        """
        Get the verification status of a model.

        Args:
            model_id: The model ID

        Returns:
            VerificationResult with current status
        """
        data = self._request("GET", f"/api/v1/verifier/status/{model_id}")
        return VerificationResult.from_dict(data)

    def test_code_visibility(
        self,
        model_id: str,
        provider: str,
        language: str = "python",
    ) -> CodeVisibilityResult:
        """
        Test if a model can see injected code.

        Args:
            model_id: The model ID
            provider: The provider name
            language: Programming language for the test code

        Returns:
            CodeVisibilityResult with test results
        """
        req = CodeVisibilityRequest(
            model_id=model_id, provider=provider, language=language
        )
        data = self._request(
            "POST", "/api/v1/verifier/test/code-visibility", json=req.to_dict()
        )
        return CodeVisibilityResult.from_dict(data)

    def reverify_model(
        self,
        model_id: str,
        provider: str,
        force: bool = True,
    ) -> VerificationResult:
        """
        Force re-verification of a model.

        Args:
            model_id: The model ID
            provider: The provider name
            force: Whether to force re-verification

        Returns:
            VerificationResult with new verification results
        """
        data = self._request(
            "POST",
            "/api/v1/verifier/reverify",
            json={"model_id": model_id, "provider": provider, "force": force},
        )
        return VerificationResult.from_dict(data)

    # Scoring methods

    def get_model_score(self, model_id: str) -> ScoreResult:
        """
        Get the score for a model.

        Args:
            model_id: The model ID

        Returns:
            ScoreResult with score details
        """
        data = self._request("GET", f"/api/v1/verifier/scores/{model_id}")
        return ScoreResult.from_dict(data)

    def batch_calculate_scores(self, model_ids: List[str]) -> List[ScoreResult]:
        """
        Calculate scores for multiple models.

        Args:
            model_ids: List of model IDs

        Returns:
            List of ScoreResult objects
        """
        data = self._request(
            "POST", "/api/v1/verifier/scores/batch", json={"model_ids": model_ids}
        )
        return [ScoreResult.from_dict(s) for s in data["scores"]]

    def get_top_models(self, limit: int = 10) -> List[ModelWithScore]:
        """
        Get the top scoring models.

        Args:
            limit: Maximum number of models to return

        Returns:
            List of ModelWithScore objects
        """
        data = self._request(
            "GET", "/api/v1/verifier/scores/top", params={"limit": limit}
        )
        return [ModelWithScore.from_dict(m) for m in data["models"]]

    def get_models_by_score_range(
        self,
        min_score: float,
        max_score: float,
        limit: int = 50,
    ) -> List[ModelWithScore]:
        """
        Get models within a score range.

        Args:
            min_score: Minimum score (0-10)
            max_score: Maximum score (0-10)
            limit: Maximum number of models

        Returns:
            List of ModelWithScore objects
        """
        data = self._request(
            "GET",
            "/api/v1/verifier/scores/range",
            params={"min_score": min_score, "max_score": max_score, "limit": limit},
        )
        return [ModelWithScore.from_dict(m) for m in data["models"]]

    def get_model_name_with_score(self, model_id: str) -> str:
        """
        Get model name with score suffix (e.g., "GPT-4 (SC:9.2)").

        Args:
            model_id: The model ID

        Returns:
            Model name with score suffix
        """
        data = self._request("GET", f"/api/v1/verifier/scores/{model_id}/name")
        return data["name_with_score"]

    def get_scoring_weights(self) -> ScoringWeights:
        """
        Get current scoring weights.

        Returns:
            ScoringWeights object
        """
        data = self._request("GET", "/api/v1/verifier/scores/weights")
        return ScoringWeights.from_dict(data["weights"])

    def update_scoring_weights(self, weights: ScoringWeights) -> ScoringWeights:
        """
        Update scoring weights.

        Args:
            weights: New weights (must sum to 1.0)

        Returns:
            Updated ScoringWeights
        """
        if not weights.validate():
            raise ValidationError("Weights must sum to 1.0")
        data = self._request(
            "PUT", "/api/v1/verifier/scores/weights", json=weights.to_dict()
        )
        return ScoringWeights.from_dict(data["weights"])

    def compare_models(self, model_ids: List[str]) -> Dict[str, Any]:
        """
        Compare multiple models.

        Args:
            model_ids: List of model IDs to compare (2-10)

        Returns:
            Comparison results including winner
        """
        return self._request(
            "POST", "/api/v1/verifier/scores/compare", json={"model_ids": model_ids}
        )

    def invalidate_cache(self, model_id: Optional[str] = None, all: bool = False):
        """
        Invalidate score cache.

        Args:
            model_id: Specific model to invalidate
            all: Invalidate all cached scores
        """
        self._request(
            "POST",
            "/api/v1/verifier/scores/cache/invalidate",
            json={"model_id": model_id, "all": all},
        )

    # Health methods

    def get_provider_health(self, provider_id: str) -> ProviderHealth:
        """
        Get health status for a provider.

        Args:
            provider_id: The provider ID

        Returns:
            ProviderHealth object
        """
        data = self._request("GET", f"/api/v1/verifier/health/providers/{provider_id}")
        return ProviderHealth.from_dict(data)

    def get_all_providers_health(self) -> List[ProviderHealth]:
        """
        Get health status for all providers.

        Returns:
            List of ProviderHealth objects
        """
        data = self._request("GET", "/api/v1/verifier/health/providers")
        return [ProviderHealth.from_dict(p) for p in data["providers"]]

    def get_healthy_providers(self) -> List[str]:
        """
        Get list of healthy provider IDs.

        Returns:
            List of healthy provider IDs
        """
        data = self._request("GET", "/api/v1/verifier/health/healthy")
        return data["providers"]

    def get_fastest_provider(self, providers: List[str]) -> Dict[str, Any]:
        """
        Get the fastest provider from a list.

        Args:
            providers: List of provider IDs

        Returns:
            Dict with provider_id and avg_response_ms
        """
        return self._request(
            "POST", "/api/v1/verifier/health/fastest", json={"providers": providers}
        )

    def is_provider_available(self, provider_id: str) -> bool:
        """
        Check if a provider is available.

        Args:
            provider_id: The provider ID

        Returns:
            True if available
        """
        data = self._request("GET", f"/api/v1/verifier/health/available/{provider_id}")
        return data["available"]

    def add_provider(self, provider_id: str, provider_name: str):
        """
        Add a provider to health monitoring.

        Args:
            provider_id: The provider ID
            provider_name: The provider name
        """
        self._request(
            "POST",
            "/api/v1/verifier/health/providers",
            json={"provider_id": provider_id, "provider_name": provider_name},
        )

    def remove_provider(self, provider_id: str):
        """
        Remove a provider from health monitoring.

        Args:
            provider_id: The provider ID
        """
        self._request("DELETE", f"/api/v1/verifier/health/providers/{provider_id}")

    # General methods

    def health(self) -> Dict[str, Any]:
        """
        Get overall verifier service health.

        Returns:
            Health status dict
        """
        return self._request("GET", "/api/v1/verifier/health")

    def get_verification_tests(self) -> Dict[str, str]:
        """
        Get available verification tests.

        Returns:
            Dict of test name to description
        """
        return self._request("GET", "/api/v1/verifier/tests")
