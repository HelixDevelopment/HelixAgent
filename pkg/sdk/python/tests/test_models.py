"""Tests for SuperAgent Verifier SDK models."""

import unittest

from superagent_verifier.models import (
    VerificationRequest,
    VerificationResult,
    BatchVerifyRequest,
    BatchVerifyResult,
    BatchVerifySummary,
    CodeVisibilityRequest,
    CodeVisibilityResult,
    ScoreComponents,
    ScoreResult,
    ModelWithScore,
    ProviderHealth,
    ScoringWeights,
)


class TestVerificationRequest(unittest.TestCase):
    """Test VerificationRequest model."""

    def test_basic_request(self):
        """Test basic request creation."""
        req = VerificationRequest(model_id="gpt-4", provider="openai")
        self.assertEqual(req.model_id, "gpt-4")
        self.assertEqual(req.provider, "openai")
        self.assertIsNone(req.tests)

    def test_request_with_tests(self):
        """Test request with tests."""
        req = VerificationRequest(
            model_id="claude-3",
            provider="anthropic",
            tests=["code_visibility", "response_quality"],
        )
        self.assertEqual(req.tests, ["code_visibility", "response_quality"])

    def test_request_with_timeout(self):
        """Test request with timeout."""
        req = VerificationRequest(
            model_id="gpt-4", provider="openai", timeout=60, retry_count=3
        )
        self.assertEqual(req.timeout, 60)
        self.assertEqual(req.retry_count, 3)

    def test_to_dict_basic(self):
        """Test to_dict basic."""
        req = VerificationRequest(model_id="gpt-4", provider="openai")
        d = req.to_dict()
        self.assertEqual(d, {"model_id": "gpt-4", "provider": "openai"})

    def test_to_dict_with_all_fields(self):
        """Test to_dict with all fields."""
        req = VerificationRequest(
            model_id="gpt-4",
            provider="openai",
            tests=["test1"],
            timeout=30,
            retry_count=5,
        )
        d = req.to_dict()
        self.assertEqual(d["tests"], ["test1"])
        self.assertEqual(d["timeout"], 30)
        self.assertEqual(d["retry_count"], 5)


class TestVerificationResult(unittest.TestCase):
    """Test VerificationResult model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "model_id": "gpt-4",
            "provider": "openai",
            "verified": True,
            "score": 9.2,
            "overall_score": 9.5,
            "score_suffix": "(SC:9.5)",
            "code_visible": True,
            "tests": {"code_visibility": True, "response_quality": True},
            "verification_time_ms": 1500,
            "message": "Verification successful",
        }
        result = VerificationResult.from_dict(data)
        self.assertEqual(result.model_id, "gpt-4")
        self.assertTrue(result.verified)
        self.assertEqual(result.overall_score, 9.5)
        self.assertTrue(result.code_visible)
        self.assertEqual(result.tests["code_visibility"], True)
        self.assertEqual(result.verification_time_ms, 1500)
        self.assertEqual(result.message, "Verification successful")

    def test_from_dict_minimal(self):
        """Test from_dict with minimal fields."""
        data = {
            "model_id": "test",
            "provider": "test",
            "verified": False,
            "score": 0,
            "overall_score": 0,
            "score_suffix": "",
            "code_visible": False,
        }
        result = VerificationResult.from_dict(data)
        self.assertEqual(result.model_id, "test")
        self.assertFalse(result.verified)
        self.assertEqual(result.tests, {})
        self.assertIsNone(result.message)


class TestBatchVerifyRequest(unittest.TestCase):
    """Test BatchVerifyRequest model."""

    def test_basic_request(self):
        """Test basic batch request."""
        models = [
            {"model_id": "gpt-4", "provider": "openai"},
            {"model_id": "claude-3", "provider": "anthropic"},
        ]
        req = BatchVerifyRequest(models=models)
        self.assertEqual(len(req.models), 2)

    def test_to_dict(self):
        """Test to_dict."""
        models = [{"model_id": "gpt-4", "provider": "openai"}]
        req = BatchVerifyRequest(models=models)
        d = req.to_dict()
        self.assertEqual(d, {"models": models})


class TestBatchVerifyResult(unittest.TestCase):
    """Test BatchVerifyResult model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "results": [
                {
                    "model_id": "gpt-4",
                    "provider": "openai",
                    "verified": True,
                    "score": 9.0,
                    "overall_score": 9.2,
                    "score_suffix": "(SC:9.2)",
                    "code_visible": True,
                },
                {
                    "model_id": "claude-3",
                    "provider": "anthropic",
                    "verified": True,
                    "score": 9.5,
                    "overall_score": 9.7,
                    "score_suffix": "(SC:9.7)",
                    "code_visible": True,
                },
            ],
            "summary": {"total": 2, "verified": 2, "failed": 0},
        }
        result = BatchVerifyResult.from_dict(data)
        self.assertEqual(len(result.results), 2)
        self.assertEqual(result.summary.total, 2)
        self.assertEqual(result.summary.verified, 2)
        self.assertEqual(result.summary.failed, 0)


class TestCodeVisibilityRequest(unittest.TestCase):
    """Test CodeVisibilityRequest model."""

    def test_default_language(self):
        """Test default language."""
        req = CodeVisibilityRequest(model_id="gpt-4", provider="openai")
        self.assertEqual(req.language, "python")

    def test_custom_language(self):
        """Test custom language."""
        req = CodeVisibilityRequest(
            model_id="gpt-4", provider="openai", language="javascript"
        )
        self.assertEqual(req.language, "javascript")

    def test_to_dict(self):
        """Test to_dict."""
        req = CodeVisibilityRequest(
            model_id="gpt-4", provider="openai", language="go"
        )
        d = req.to_dict()
        self.assertEqual(d["model_id"], "gpt-4")
        self.assertEqual(d["provider"], "openai")
        self.assertEqual(d["language"], "go")


class TestCodeVisibilityResult(unittest.TestCase):
    """Test CodeVisibilityResult model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "model_id": "gpt-4",
            "provider": "openai",
            "code_visible": True,
            "language": "python",
            "prompt": "Do you see my code?",
            "response": "Yes, I can see your Python code.",
            "confidence": 0.95,
        }
        result = CodeVisibilityResult.from_dict(data)
        self.assertEqual(result.model_id, "gpt-4")
        self.assertTrue(result.code_visible)
        self.assertEqual(result.language, "python")
        self.assertEqual(result.confidence, 0.95)


class TestScoreComponents(unittest.TestCase):
    """Test ScoreComponents model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "speed_score": 9.0,
            "efficiency_score": 8.5,
            "cost_score": 7.0,
            "capability_score": 9.5,
            "recency_score": 8.0,
        }
        components = ScoreComponents.from_dict(data)
        self.assertEqual(components.speed_score, 9.0)
        self.assertEqual(components.efficiency_score, 8.5)
        self.assertEqual(components.cost_score, 7.0)
        self.assertEqual(components.capability_score, 9.5)
        self.assertEqual(components.recency_score, 8.0)


class TestScoreResult(unittest.TestCase):
    """Test ScoreResult model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "model_id": "gpt-4",
            "model_name": "GPT-4",
            "overall_score": 9.2,
            "score_suffix": "(SC:9.2)",
            "components": {
                "speed_score": 9.0,
                "efficiency_score": 8.5,
                "cost_score": 7.0,
                "capability_score": 9.5,
                "recency_score": 8.0,
            },
            "calculated_at": "2024-01-15T10:30:00Z",
            "data_source": "models.dev",
        }
        result = ScoreResult.from_dict(data)
        self.assertEqual(result.model_id, "gpt-4")
        self.assertEqual(result.model_name, "GPT-4")
        self.assertEqual(result.overall_score, 9.2)
        self.assertEqual(result.components.speed_score, 9.0)
        self.assertEqual(result.data_source, "models.dev")


class TestModelWithScore(unittest.TestCase):
    """Test ModelWithScore model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "model_id": "gpt-4",
            "name": "GPT-4",
            "provider": "openai",
            "overall_score": 9.5,
            "score_suffix": "(SC:9.5)",
            "rank": 1,
        }
        model = ModelWithScore.from_dict(data)
        self.assertEqual(model.model_id, "gpt-4")
        self.assertEqual(model.name, "GPT-4")
        self.assertEqual(model.provider, "openai")
        self.assertEqual(model.overall_score, 9.5)
        self.assertEqual(model.rank, 1)

    def test_from_dict_without_rank(self):
        """Test from_dict without rank."""
        data = {
            "model_id": "test",
            "name": "Test",
            "provider": "test",
            "overall_score": 5.0,
            "score_suffix": "(SC:5.0)",
        }
        model = ModelWithScore.from_dict(data)
        self.assertEqual(model.rank, 0)


class TestProviderHealth(unittest.TestCase):
    """Test ProviderHealth model."""

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "provider_id": "openai",
            "provider_name": "OpenAI",
            "healthy": True,
            "circuit_state": "closed",
            "failure_count": 0,
            "success_count": 100,
            "avg_response_ms": 250,
            "uptime_percent": 99.9,
            "last_checked_at": "2024-01-15T10:30:00Z",
            "last_success_at": "2024-01-15T10:30:00Z",
            "last_failure_at": None,
        }
        health = ProviderHealth.from_dict(data)
        self.assertEqual(health.provider_id, "openai")
        self.assertTrue(health.healthy)
        self.assertEqual(health.circuit_state, "closed")
        self.assertEqual(health.avg_response_ms, 250)
        self.assertEqual(health.uptime_percent, 99.9)
        self.assertIsNone(health.last_failure_at)

    def test_from_dict_minimal(self):
        """Test from_dict with minimal fields."""
        data = {
            "provider_id": "test",
            "provider_name": "Test",
            "healthy": False,
            "circuit_state": "open",
            "failure_count": 5,
            "success_count": 0,
            "avg_response_ms": 0,
            "uptime_percent": 0,
            "last_checked_at": "2024-01-15T10:30:00Z",
        }
        health = ProviderHealth.from_dict(data)
        self.assertFalse(health.healthy)
        self.assertEqual(health.circuit_state, "open")


class TestScoringWeights(unittest.TestCase):
    """Test ScoringWeights model."""

    def test_default(self):
        """Test default weights."""
        weights = ScoringWeights.default()
        self.assertEqual(weights.response_speed, 0.25)
        self.assertEqual(weights.model_efficiency, 0.20)
        self.assertEqual(weights.cost_effectiveness, 0.25)
        self.assertEqual(weights.capability, 0.20)
        self.assertEqual(weights.recency, 0.10)

    def test_validate_valid(self):
        """Test validate with valid weights."""
        weights = ScoringWeights.default()
        self.assertTrue(weights.validate())

    def test_validate_invalid(self):
        """Test validate with invalid weights."""
        weights = ScoringWeights(
            response_speed=0.5,
            model_efficiency=0.5,
            cost_effectiveness=0.5,
            capability=0.5,
            recency=0.5,
        )
        self.assertFalse(weights.validate())

    def test_to_dict(self):
        """Test to_dict."""
        weights = ScoringWeights.default()
        d = weights.to_dict()
        self.assertEqual(d["response_speed"], 0.25)
        self.assertEqual(d["model_efficiency"], 0.20)
        self.assertEqual(d["cost_effectiveness"], 0.25)

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "response_speed": 0.3,
            "model_efficiency": 0.2,
            "cost_effectiveness": 0.2,
            "capability": 0.2,
            "recency": 0.1,
        }
        weights = ScoringWeights.from_dict(data)
        self.assertEqual(weights.response_speed, 0.3)
        self.assertTrue(weights.validate())


if __name__ == "__main__":
    unittest.main()
