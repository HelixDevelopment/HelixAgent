"""Tests for SuperAgent client."""

import json
import os
import unittest
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread
from unittest.mock import patch

from superagent import SuperAgent
from superagent.exceptions import AuthenticationError, APIError
from superagent.types import ChatMessage


class MockHandler(BaseHTTPRequestHandler):
    """Mock HTTP handler for testing."""

    def log_message(self, format, *args):
        """Suppress log output."""
        pass

    def do_GET(self):
        """Handle GET requests."""
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"status": "healthy"}).encode())
        elif self.path == "/v1/models":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({
                "data": [
                    {"id": "gpt-4", "object": "model", "created": 1234567890, "owned_by": "openai"},
                    {"id": "claude-3", "object": "model", "created": 1234567890, "owned_by": "anthropic"},
                ]
            }).encode())
        elif self.path == "/v1/providers":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({
                "providers": ["openai", "anthropic", "google"]
            }).encode())
        else:
            self.send_response(404)
            self.end_headers()

    def do_POST(self):
        """Handle POST requests."""
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        data = json.loads(body) if body else {}

        if self.path == "/v1/chat/completions":
            # Check authorization
            auth = self.headers.get("Authorization", "")
            if not auth.startswith("Bearer "):
                self.send_response(401)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps({"error": {"message": "Unauthorized"}}).encode())
                return

            # Return mock completion
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            response = {
                "id": "chatcmpl-123",
                "object": "chat.completion",
                "created": 1234567890,
                "model": data.get("model", "superagent-ensemble"),
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": "Hello! How can I help you today?"
                        },
                        "finish_reason": "stop"
                    }
                ],
                "usage": {
                    "prompt_tokens": 10,
                    "completion_tokens": 8,
                    "total_tokens": 18
                }
            }
            self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(404)
            self.end_headers()


class TestSuperAgentClient(unittest.TestCase):
    """Test SuperAgent client."""

    @classmethod
    def setUpClass(cls):
        """Start mock server."""
        cls.server = HTTPServer(("localhost", 0), MockHandler)
        cls.port = cls.server.server_address[1]
        cls.server_thread = Thread(target=cls.server.serve_forever)
        cls.server_thread.daemon = True
        cls.server_thread.start()

    @classmethod
    def tearDownClass(cls):
        """Stop mock server."""
        cls.server.shutdown()

    def test_client_initialization(self):
        """Test client initialization."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        self.assertEqual(client.api_key, "test-key")
        self.assertEqual(client.base_url, f"http://localhost:{self.port}")

    def test_client_initialization_from_env(self):
        """Test client initialization from environment."""
        with patch.dict(os.environ, {"SUPERAGENT_API_KEY": "env-key"}):
            client = SuperAgent(base_url=f"http://localhost:{self.port}")
            self.assertEqual(client.api_key, "env-key")

    def test_health_check(self):
        """Test health check endpoint."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        health = client.health()
        self.assertEqual(health["status"], "healthy")

    def test_list_models(self):
        """Test listing models."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        models = client.models.list()
        self.assertEqual(len(models), 2)
        self.assertEqual(models[0].id, "gpt-4")
        self.assertEqual(models[1].id, "claude-3")

    def test_list_providers(self):
        """Test listing providers."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        providers = client.providers()
        self.assertEqual(providers, ["openai", "anthropic", "google"])

    def test_chat_completion(self):
        """Test chat completion."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        response = client.chat.completions.create(
            model="superagent-ensemble",
            messages=[{"role": "user", "content": "Hello!"}]
        )
        self.assertEqual(response.id, "chatcmpl-123")
        self.assertEqual(len(response.choices), 1)
        self.assertEqual(response.choices[0].message.role, "assistant")
        self.assertIn("Hello", response.choices[0].message.content)

    def test_chat_completion_with_chat_message(self):
        """Test chat completion with ChatMessage objects."""
        client = SuperAgent(
            api_key="test-key",
            base_url=f"http://localhost:{self.port}"
        )
        response = client.chat.completions.create(
            model="superagent-ensemble",
            messages=[ChatMessage(role="user", content="Hello!")]
        )
        self.assertEqual(response.id, "chatcmpl-123")

    def test_chat_completion_unauthorized(self):
        """Test chat completion without auth."""
        client = SuperAgent(
            base_url=f"http://localhost:{self.port}"
        )
        with self.assertRaises(AuthenticationError):
            client.chat.completions.create(
                model="superagent-ensemble",
                messages=[{"role": "user", "content": "Hello!"}]
            )


class TestChatMessage(unittest.TestCase):
    """Test ChatMessage type."""

    def test_to_dict(self):
        """Test ChatMessage to dict conversion."""
        msg = ChatMessage(role="user", content="Hello!")
        d = msg.to_dict()
        self.assertEqual(d["role"], "user")
        self.assertEqual(d["content"], "Hello!")

    def test_to_dict_with_name(self):
        """Test ChatMessage to dict with name."""
        msg = ChatMessage(role="user", content="Hello!", name="Alice")
        d = msg.to_dict()
        self.assertEqual(d["name"], "Alice")

    def test_from_dict(self):
        """Test ChatMessage from dict."""
        d = {"role": "assistant", "content": "Hi there!"}
        msg = ChatMessage.from_dict(d)
        self.assertEqual(msg.role, "assistant")
        self.assertEqual(msg.content, "Hi there!")


if __name__ == "__main__":
    unittest.main()
