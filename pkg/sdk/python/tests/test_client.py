"""Tests for SuperAgent Verifier SDK client."""

import json
import unittest
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread
from unittest.mock import patch, MagicMock

from superagent_verifier.client import VerifierClient
from superagent_verifier.models import ScoringWeights
from superagent_verifier.exceptions import (
    APIError,
    AuthenticationError,
    NotFoundError,
    ValidationError,
)


class MockVerifierHandler(BaseHTTPRequestHandler):
    """Mock HTTP handler for verifier API."""

    def log_message(self, format, *args):
        """Suppress log output."""
        pass

    def _send_json(self, status_code, data):
        """Helper to send JSON response."""
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

    def _check_auth(self):
        """Check authorization header."""
        auth = self.headers.get("Authorization", "")
        if auth == "Bearer invalid-key":
            self._send_json(401, {"error": "Authentication failed"})
            return False
        return True

    def do_GET(self):
        """Handle GET requests."""
        if not self._check_auth():
            return

        # Parse path and query string
        path = self.path.split("?")[0]

        if path == "/api/v1/verifier/health":
            self._send_json(200, {"status": "healthy", "version": "1.0.0"})

        elif path.startswith("/api/v1/verifier/status/"):
            model_id = path.split("/")[-1]
            if model_id == "unknown-model":
                self._send_json(404, {"error": "Model not found"})
            else:
                self._send_json(200, {
                    "model_id": model_id,
                    "provider": "openai",
                    "verified": True,
                    "score": 9.0,
                    "overall_score": 9.2,
                    "score_suffix": "(SC:9.2)",
                    "code_visible": True,
                    "tests": {"code_visibility": True},
                })

        elif path == "/api/v1/verifier/scores/weights":
            self._send_json(200, {
                "weights": {
                    "response_speed": 0.25,
                    "model_efficiency": 0.20,
                    "cost_effectiveness": 0.25,
                    "capability": 0.20,
                    "recency": 0.10,
                }
            })

        elif path == "/api/v1/verifier/scores/top":
            self._send_json(200, {
                "models": [
                    {"model_id": "gpt-4", "name": "GPT-4", "provider": "openai",
                     "overall_score": 9.5, "score_suffix": "(SC:9.5)", "rank": 1},
                    {"model_id": "claude-3", "name": "Claude 3", "provider": "anthropic",
                     "overall_score": 9.3, "score_suffix": "(SC:9.3)", "rank": 2},
                ]
            })

        elif path == "/api/v1/verifier/scores/range":
            self._send_json(200, {
                "models": [
                    {"model_id": "gpt-4", "name": "GPT-4", "provider": "openai",
                     "overall_score": 8.5, "score_suffix": "(SC:8.5)", "rank": 1},
                ]
            })

        elif path.startswith("/api/v1/verifier/scores/") and path.endswith("/name"):
            self._send_json(200, {"name_with_score": "GPT-4 (SC:9.2)"})

        elif path.startswith("/api/v1/verifier/scores/"):
            model_id = path.split("/")[-1]
            self._send_json(200, {
                "model_id": model_id,
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
            })

        elif path.startswith("/api/v1/verifier/health/providers/"):
            provider_id = path.split("/")[-1]
            self._send_json(200, {
                "provider_id": provider_id,
                "provider_name": provider_id.title(),
                "healthy": True,
                "circuit_state": "closed",
                "failure_count": 0,
                "success_count": 100,
                "avg_response_ms": 250,
                "uptime_percent": 99.9,
                "last_checked_at": "2024-01-15T10:30:00Z",
            })

        elif path == "/api/v1/verifier/health/providers":
            self._send_json(200, {
                "providers": [
                    {"provider_id": "openai", "provider_name": "OpenAI", "healthy": True,
                     "circuit_state": "closed", "failure_count": 0, "success_count": 100,
                     "avg_response_ms": 250, "uptime_percent": 99.9, "last_checked_at": "2024-01-15T10:30:00Z"},
                ]
            })

        elif path == "/api/v1/verifier/health/healthy":
            self._send_json(200, {"providers": ["openai", "anthropic", "google"]})

        elif path.startswith("/api/v1/verifier/health/available/"):
            provider_id = path.split("/")[-1]
            self._send_json(200, {"available": provider_id != "unavailable"})

        elif path == "/api/v1/verifier/tests":
            self._send_json(200, {
                "code_visibility": "Tests if model can see injected code",
                "response_quality": "Tests response quality metrics",
            })

        else:
            self._send_json(404, {"error": "Not found"})

    def do_POST(self):
        """Handle POST requests."""
        if not self._check_auth():
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        data = json.loads(body) if body else {}

        if self.path == "/api/v1/verifier/verify":
            model_id = data.get("model_id", "unknown")
            if data.get("model_id") == "invalid-model":
                self._send_json(400, {"error": "Invalid model ID"})
            else:
                self._send_json(200, {
                    "model_id": model_id,
                    "provider": data.get("provider", "unknown"),
                    "verified": True,
                    "score": 9.0,
                    "overall_score": 9.2,
                    "score_suffix": "(SC:9.2)",
                    "code_visible": True,
                    "tests": {"code_visibility": True},
                    "verification_time_ms": 1500,
                })

        elif self.path == "/api/v1/verifier/verify/batch":
            models = data.get("models", [])
            results = []
            for m in models:
                results.append({
                    "model_id": m["model_id"],
                    "provider": m["provider"],
                    "verified": True,
                    "score": 9.0,
                    "overall_score": 9.2,
                    "score_suffix": "(SC:9.2)",
                    "code_visible": True,
                })
            self._send_json(200, {
                "results": results,
                "summary": {"total": len(models), "verified": len(models), "failed": 0},
            })

        elif self.path == "/api/v1/verifier/test/code-visibility":
            self._send_json(200, {
                "model_id": data["model_id"],
                "provider": data["provider"],
                "code_visible": True,
                "language": data.get("language", "python"),
                "prompt": "Do you see my code?",
                "response": "Yes, I can see your code.",
                "confidence": 0.95,
            })

        elif self.path == "/api/v1/verifier/reverify":
            self._send_json(200, {
                "model_id": data["model_id"],
                "provider": data["provider"],
                "verified": True,
                "score": 9.5,
                "overall_score": 9.7,
                "score_suffix": "(SC:9.7)",
                "code_visible": True,
                "tests": {"code_visibility": True},
                "verification_time_ms": 2000,
            })

        elif self.path == "/api/v1/verifier/scores/batch":
            scores = []
            for model_id in data.get("model_ids", []):
                scores.append({
                    "model_id": model_id,
                    "model_name": model_id.upper(),
                    "overall_score": 9.0,
                    "score_suffix": "(SC:9.0)",
                    "components": {
                        "speed_score": 9.0,
                        "efficiency_score": 8.5,
                        "cost_score": 7.0,
                        "capability_score": 9.5,
                        "recency_score": 8.0,
                    },
                    "calculated_at": "2024-01-15T10:30:00Z",
                    "data_source": "models.dev",
                })
            self._send_json(200, {"scores": scores})

        elif self.path == "/api/v1/verifier/scores/compare":
            self._send_json(200, {
                "models": data["model_ids"],
                "winner": data["model_ids"][0],
                "comparison": {"gpt-4": 9.5, "claude-3": 9.3},
            })

        elif self.path == "/api/v1/verifier/scores/cache/invalidate":
            self._send_json(200, {"success": True})

        elif self.path == "/api/v1/verifier/health/fastest":
            self._send_json(200, {
                "provider_id": data["providers"][0],
                "avg_response_ms": 150,
            })

        elif self.path == "/api/v1/verifier/health/providers":
            self._send_json(200, {"success": True})

        else:
            self._send_json(404, {"error": "Not found"})

    def do_PUT(self):
        """Handle PUT requests."""
        if not self._check_auth():
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        data = json.loads(body) if body else {}

        if self.path == "/api/v1/verifier/scores/weights":
            self._send_json(200, {"weights": data})
        else:
            self._send_json(404, {"error": "Not found"})

    def do_DELETE(self):
        """Handle DELETE requests."""
        if not self._check_auth():
            return

        if self.path.startswith("/api/v1/verifier/health/providers/"):
            self._send_json(200, {"success": True})
        else:
            self._send_json(404, {"error": "Not found"})


class TestVerifierClient(unittest.TestCase):
    """Test VerifierClient."""

    @classmethod
    def setUpClass(cls):
        """Start mock server."""
        cls.server = HTTPServer(("localhost", 0), MockVerifierHandler)
        cls.port = cls.server.server_address[1]
        cls.server_thread = Thread(target=cls.server.serve_forever)
        cls.server_thread.daemon = True
        cls.server_thread.start()
        cls.base_url = f"http://localhost:{cls.port}"

    @classmethod
    def tearDownClass(cls):
        """Stop mock server."""
        cls.server.shutdown()

    def get_client(self, api_key="test-key"):
        """Get a client instance."""
        return VerifierClient(base_url=self.base_url, api_key=api_key)

    def test_client_initialization(self):
        """Test client initialization."""
        client = self.get_client()
        self.assertEqual(client.base_url, self.base_url)
        self.assertEqual(client.api_key, "test-key")
        self.assertEqual(client.timeout, 30)

    def test_client_custom_timeout(self):
        """Test client with custom timeout."""
        client = VerifierClient(base_url=self.base_url, timeout=60)
        self.assertEqual(client.timeout, 60)

    # Health tests
    def test_health(self):
        """Test health endpoint."""
        client = self.get_client()
        health = client.health()
        self.assertEqual(health["status"], "healthy")

    # Verification tests
    def test_verify_model(self):
        """Test verify model."""
        client = self.get_client()
        result = client.verify_model("gpt-4", "openai")
        self.assertEqual(result.model_id, "gpt-4")
        self.assertTrue(result.verified)
        self.assertEqual(result.overall_score, 9.2)
        self.assertTrue(result.code_visible)

    def test_verify_model_with_tests(self):
        """Test verify model with specific tests."""
        client = self.get_client()
        result = client.verify_model("gpt-4", "openai", tests=["code_visibility"])
        self.assertTrue(result.verified)

    def test_verify_model_invalid(self):
        """Test verify model with invalid model."""
        client = self.get_client()
        with self.assertRaises(ValidationError):
            client.verify_model("invalid-model", "openai")

    def test_batch_verify(self):
        """Test batch verification."""
        client = self.get_client()
        models = [
            {"model_id": "gpt-4", "provider": "openai"},
            {"model_id": "claude-3", "provider": "anthropic"},
        ]
        result = client.batch_verify(models)
        self.assertEqual(len(result.results), 2)
        self.assertEqual(result.summary.total, 2)
        self.assertEqual(result.summary.verified, 2)

    def test_get_verification_status(self):
        """Test get verification status."""
        client = self.get_client()
        result = client.get_verification_status("gpt-4")
        self.assertEqual(result.model_id, "gpt-4")
        self.assertTrue(result.verified)

    def test_test_code_visibility(self):
        """Test code visibility test."""
        client = self.get_client()
        result = client.test_code_visibility("gpt-4", "openai", language="python")
        self.assertEqual(result.model_id, "gpt-4")
        self.assertTrue(result.code_visible)
        self.assertEqual(result.language, "python")
        self.assertEqual(result.confidence, 0.95)

    def test_reverify_model(self):
        """Test reverify model."""
        client = self.get_client()
        result = client.reverify_model("gpt-4", "openai", force=True)
        self.assertTrue(result.verified)
        self.assertEqual(result.overall_score, 9.7)

    # Scoring tests
    def test_get_model_score(self):
        """Test get model score."""
        client = self.get_client()
        result = client.get_model_score("gpt-4")
        self.assertEqual(result.model_id, "gpt-4")
        self.assertEqual(result.overall_score, 9.2)
        self.assertEqual(result.components.speed_score, 9.0)

    def test_batch_calculate_scores(self):
        """Test batch calculate scores."""
        client = self.get_client()
        results = client.batch_calculate_scores(["gpt-4", "claude-3"])
        self.assertEqual(len(results), 2)

    def test_get_top_models(self):
        """Test get top models."""
        client = self.get_client()
        models = client.get_top_models(limit=10)
        self.assertEqual(len(models), 2)
        self.assertEqual(models[0].model_id, "gpt-4")
        self.assertEqual(models[0].rank, 1)

    def test_get_models_by_score_range(self):
        """Test get models by score range."""
        client = self.get_client()
        models = client.get_models_by_score_range(8.0, 9.0, limit=50)
        self.assertEqual(len(models), 1)

    def test_get_model_name_with_score(self):
        """Test get model name with score."""
        client = self.get_client()
        name = client.get_model_name_with_score("gpt-4")
        self.assertEqual(name, "GPT-4 (SC:9.2)")

    def test_get_scoring_weights(self):
        """Test get scoring weights."""
        client = self.get_client()
        weights = client.get_scoring_weights()
        self.assertEqual(weights.response_speed, 0.25)
        self.assertTrue(weights.validate())

    def test_update_scoring_weights(self):
        """Test update scoring weights."""
        client = self.get_client()
        weights = ScoringWeights.default()
        updated = client.update_scoring_weights(weights)
        self.assertEqual(updated.response_speed, 0.25)

    def test_update_scoring_weights_invalid(self):
        """Test update scoring weights with invalid weights."""
        client = self.get_client()
        weights = ScoringWeights(
            response_speed=0.5,
            model_efficiency=0.5,
            cost_effectiveness=0.5,
            capability=0.5,
            recency=0.5,
        )
        with self.assertRaises(ValidationError):
            client.update_scoring_weights(weights)

    def test_compare_models(self):
        """Test compare models."""
        client = self.get_client()
        result = client.compare_models(["gpt-4", "claude-3"])
        self.assertEqual(result["winner"], "gpt-4")

    def test_invalidate_cache(self):
        """Test invalidate cache."""
        client = self.get_client()
        # Should not raise
        client.invalidate_cache(model_id="gpt-4")
        client.invalidate_cache(all=True)

    # Health tests
    def test_get_provider_health(self):
        """Test get provider health."""
        client = self.get_client()
        health = client.get_provider_health("openai")
        self.assertEqual(health.provider_id, "openai")
        self.assertTrue(health.healthy)
        self.assertEqual(health.circuit_state, "closed")

    def test_get_all_providers_health(self):
        """Test get all providers health."""
        client = self.get_client()
        providers = client.get_all_providers_health()
        self.assertEqual(len(providers), 1)

    def test_get_healthy_providers(self):
        """Test get healthy providers."""
        client = self.get_client()
        providers = client.get_healthy_providers()
        self.assertEqual(providers, ["openai", "anthropic", "google"])

    def test_get_fastest_provider(self):
        """Test get fastest provider."""
        client = self.get_client()
        result = client.get_fastest_provider(["openai", "anthropic"])
        self.assertEqual(result["provider_id"], "openai")

    def test_is_provider_available(self):
        """Test is provider available."""
        client = self.get_client()
        self.assertTrue(client.is_provider_available("openai"))
        self.assertFalse(client.is_provider_available("unavailable"))

    def test_add_provider(self):
        """Test add provider."""
        client = self.get_client()
        # Should not raise
        client.add_provider("new-provider", "New Provider")

    def test_remove_provider(self):
        """Test remove provider."""
        client = self.get_client()
        # Should not raise
        client.remove_provider("openai")

    def test_get_verification_tests(self):
        """Test get verification tests."""
        client = self.get_client()
        tests = client.get_verification_tests()
        self.assertIn("code_visibility", tests)
        self.assertIn("response_quality", tests)

    # Error handling tests
    def test_authentication_error(self):
        """Test authentication error."""
        client = VerifierClient(base_url=self.base_url, api_key="invalid-key")
        with self.assertRaises(AuthenticationError):
            client.health()


class TestVerifierClientRequestErrors(unittest.TestCase):
    """Test VerifierClient request error handling."""

    def test_timeout_error(self):
        """Test timeout error handling."""
        client = VerifierClient(base_url="http://10.255.255.1", timeout=1)
        with self.assertRaises(APIError) as ctx:
            client.health()
        # Should get a connection error, not necessarily timeout
        self.assertIsNotNone(ctx.exception)

    def test_connection_error(self):
        """Test connection error handling."""
        client = VerifierClient(base_url="http://localhost:1", timeout=1)
        with self.assertRaises(APIError):
            client.health()


if __name__ == "__main__":
    unittest.main()
