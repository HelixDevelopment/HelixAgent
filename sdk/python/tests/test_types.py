"""Tests for SuperAgent SDK types."""

import unittest
from datetime import datetime

from superagent.types import (
    ChatMessage,
    Usage,
    ChatCompletionChoice,
    ChatCompletionResponse,
    StreamDelta,
    StreamChoice,
    ChatCompletionChunk,
    Model,
    EnsembleConfig,
)


class TestChatMessage(unittest.TestCase):
    """Test ChatMessage type."""

    def test_basic_message(self):
        """Test basic message creation."""
        msg = ChatMessage(role="user", content="Hello!")
        self.assertEqual(msg.role, "user")
        self.assertEqual(msg.content, "Hello!")
        self.assertIsNone(msg.name)
        self.assertIsNone(msg.tool_calls)

    def test_message_with_name(self):
        """Test message with name."""
        msg = ChatMessage(role="user", content="Hello!", name="Alice")
        self.assertEqual(msg.name, "Alice")

    def test_message_with_tool_calls(self):
        """Test message with tool calls."""
        tool_calls = [{"id": "call_123", "type": "function"}]
        msg = ChatMessage(role="assistant", content="", tool_calls=tool_calls)
        self.assertEqual(msg.tool_calls, tool_calls)

    def test_to_dict_basic(self):
        """Test to_dict for basic message."""
        msg = ChatMessage(role="user", content="Hello!")
        d = msg.to_dict()
        self.assertEqual(d, {"role": "user", "content": "Hello!"})

    def test_to_dict_with_name(self):
        """Test to_dict with name."""
        msg = ChatMessage(role="user", content="Hello!", name="Alice")
        d = msg.to_dict()
        self.assertEqual(d["name"], "Alice")

    def test_to_dict_with_tool_calls(self):
        """Test to_dict with tool calls."""
        tool_calls = [{"id": "call_123"}]
        msg = ChatMessage(role="assistant", content="", tool_calls=tool_calls)
        d = msg.to_dict()
        self.assertEqual(d["tool_calls"], tool_calls)

    def test_from_dict_basic(self):
        """Test from_dict for basic message."""
        data = {"role": "assistant", "content": "Hi there!"}
        msg = ChatMessage.from_dict(data)
        self.assertEqual(msg.role, "assistant")
        self.assertEqual(msg.content, "Hi there!")

    def test_from_dict_with_all_fields(self):
        """Test from_dict with all fields."""
        data = {
            "role": "assistant",
            "content": "Hello!",
            "name": "Bot",
            "tool_calls": [{"id": "123"}],
        }
        msg = ChatMessage.from_dict(data)
        self.assertEqual(msg.name, "Bot")
        self.assertEqual(msg.tool_calls, [{"id": "123"}])

    def test_from_dict_empty(self):
        """Test from_dict with empty dict."""
        msg = ChatMessage.from_dict({})
        self.assertEqual(msg.role, "")
        self.assertEqual(msg.content, "")


class TestUsage(unittest.TestCase):
    """Test Usage type."""

    def test_default_values(self):
        """Test default values."""
        usage = Usage()
        self.assertEqual(usage.prompt_tokens, 0)
        self.assertEqual(usage.completion_tokens, 0)
        self.assertEqual(usage.total_tokens, 0)

    def test_with_values(self):
        """Test with values."""
        usage = Usage(prompt_tokens=10, completion_tokens=20, total_tokens=30)
        self.assertEqual(usage.prompt_tokens, 10)
        self.assertEqual(usage.completion_tokens, 20)
        self.assertEqual(usage.total_tokens, 30)

    def test_from_dict(self):
        """Test from_dict."""
        data = {"prompt_tokens": 100, "completion_tokens": 200, "total_tokens": 300}
        usage = Usage.from_dict(data)
        self.assertEqual(usage.prompt_tokens, 100)
        self.assertEqual(usage.completion_tokens, 200)
        self.assertEqual(usage.total_tokens, 300)

    def test_from_dict_empty(self):
        """Test from_dict with empty dict."""
        usage = Usage.from_dict({})
        self.assertEqual(usage.prompt_tokens, 0)
        self.assertEqual(usage.total_tokens, 0)


class TestChatCompletionChoice(unittest.TestCase):
    """Test ChatCompletionChoice type."""

    def test_basic_choice(self):
        """Test basic choice creation."""
        msg = ChatMessage(role="assistant", content="Hello!")
        choice = ChatCompletionChoice(index=0, message=msg, finish_reason="stop")
        self.assertEqual(choice.index, 0)
        self.assertEqual(choice.message.content, "Hello!")
        self.assertEqual(choice.finish_reason, "stop")

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "index": 1,
            "message": {"role": "assistant", "content": "Response"},
            "finish_reason": "length",
        }
        choice = ChatCompletionChoice.from_dict(data)
        self.assertEqual(choice.index, 1)
        self.assertEqual(choice.message.content, "Response")
        self.assertEqual(choice.finish_reason, "length")

    def test_from_dict_defaults(self):
        """Test from_dict with missing fields."""
        choice = ChatCompletionChoice.from_dict({})
        self.assertEqual(choice.index, 0)
        self.assertIsNone(choice.finish_reason)


class TestChatCompletionResponse(unittest.TestCase):
    """Test ChatCompletionResponse type."""

    def test_basic_response(self):
        """Test basic response creation."""
        msg = ChatMessage(role="assistant", content="Hello!")
        choice = ChatCompletionChoice(index=0, message=msg)
        response = ChatCompletionResponse(
            id="chatcmpl-123",
            object="chat.completion",
            created=1234567890,
            model="gpt-4",
            choices=[choice],
        )
        self.assertEqual(response.id, "chatcmpl-123")
        self.assertEqual(response.model, "gpt-4")
        self.assertEqual(len(response.choices), 1)

    def test_response_with_usage(self):
        """Test response with usage."""
        msg = ChatMessage(role="assistant", content="Hello!")
        choice = ChatCompletionChoice(index=0, message=msg)
        usage = Usage(prompt_tokens=10, completion_tokens=5, total_tokens=15)
        response = ChatCompletionResponse(
            id="chatcmpl-123",
            object="chat.completion",
            created=1234567890,
            model="gpt-4",
            choices=[choice],
            usage=usage,
        )
        self.assertIsNotNone(response.usage)
        self.assertEqual(response.usage.total_tokens, 15)

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "id": "chatcmpl-456",
            "object": "chat.completion",
            "created": 1234567890,
            "model": "superagent-ensemble",
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": "Hi!"},
                    "finish_reason": "stop",
                }
            ],
            "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
        }
        response = ChatCompletionResponse.from_dict(data)
        self.assertEqual(response.id, "chatcmpl-456")
        self.assertEqual(response.model, "superagent-ensemble")
        self.assertEqual(len(response.choices), 1)
        self.assertEqual(response.choices[0].message.content, "Hi!")
        self.assertEqual(response.usage.total_tokens, 15)

    def test_from_dict_no_usage(self):
        """Test from_dict without usage."""
        data = {
            "id": "chatcmpl-789",
            "object": "chat.completion",
            "created": 1234567890,
            "model": "gpt-3.5-turbo",
            "choices": [],
        }
        response = ChatCompletionResponse.from_dict(data)
        self.assertIsNone(response.usage)


class TestStreamDelta(unittest.TestCase):
    """Test StreamDelta type."""

    def test_default_values(self):
        """Test default values."""
        delta = StreamDelta()
        self.assertIsNone(delta.role)
        self.assertIsNone(delta.content)

    def test_with_content(self):
        """Test with content."""
        delta = StreamDelta(content="Hello")
        self.assertEqual(delta.content, "Hello")

    def test_with_role(self):
        """Test with role."""
        delta = StreamDelta(role="assistant")
        self.assertEqual(delta.role, "assistant")

    def test_from_dict(self):
        """Test from_dict."""
        delta = StreamDelta.from_dict({"role": "assistant", "content": "Hi"})
        self.assertEqual(delta.role, "assistant")
        self.assertEqual(delta.content, "Hi")

    def test_from_dict_empty(self):
        """Test from_dict with empty dict."""
        delta = StreamDelta.from_dict({})
        self.assertIsNone(delta.role)
        self.assertIsNone(delta.content)


class TestStreamChoice(unittest.TestCase):
    """Test StreamChoice type."""

    def test_basic_choice(self):
        """Test basic choice."""
        delta = StreamDelta(content="token")
        choice = StreamChoice(index=0, delta=delta)
        self.assertEqual(choice.index, 0)
        self.assertEqual(choice.delta.content, "token")

    def test_from_dict(self):
        """Test from_dict."""
        data = {"index": 1, "delta": {"content": "chunk"}, "finish_reason": "stop"}
        choice = StreamChoice.from_dict(data)
        self.assertEqual(choice.index, 1)
        self.assertEqual(choice.delta.content, "chunk")
        self.assertEqual(choice.finish_reason, "stop")


class TestChatCompletionChunk(unittest.TestCase):
    """Test ChatCompletionChunk type."""

    def test_basic_chunk(self):
        """Test basic chunk creation."""
        delta = StreamDelta(content="Hello")
        choice = StreamChoice(index=0, delta=delta)
        chunk = ChatCompletionChunk(
            id="chatcmpl-123",
            object="chat.completion.chunk",
            created=1234567890,
            model="gpt-4",
            choices=[choice],
        )
        self.assertEqual(chunk.id, "chatcmpl-123")
        self.assertEqual(chunk.model, "gpt-4")

    def test_from_dict(self):
        """Test from_dict."""
        data = {
            "id": "chatcmpl-stream-123",
            "object": "chat.completion.chunk",
            "created": 1234567890,
            "model": "gpt-4",
            "choices": [{"index": 0, "delta": {"content": "Hello"}}],
        }
        chunk = ChatCompletionChunk.from_dict(data)
        self.assertEqual(chunk.id, "chatcmpl-stream-123")
        self.assertEqual(chunk.choices[0].delta.content, "Hello")


class TestModel(unittest.TestCase):
    """Test Model type."""

    def test_basic_model(self):
        """Test basic model."""
        model = Model(id="gpt-4")
        self.assertEqual(model.id, "gpt-4")
        self.assertEqual(model.object, "model")

    def test_model_with_all_fields(self):
        """Test model with all fields."""
        model = Model(id="gpt-4", object="model", created=1234567890, owned_by="openai")
        self.assertEqual(model.owned_by, "openai")
        self.assertEqual(model.created, 1234567890)

    def test_from_dict(self):
        """Test from_dict."""
        data = {"id": "claude-3", "object": "model", "created": 1234567890, "owned_by": "anthropic"}
        model = Model.from_dict(data)
        self.assertEqual(model.id, "claude-3")
        self.assertEqual(model.owned_by, "anthropic")

    def test_from_dict_defaults(self):
        """Test from_dict with defaults."""
        model = Model.from_dict({"id": "test-model"})
        self.assertEqual(model.object, "model")
        self.assertEqual(model.created, 0)
        self.assertEqual(model.owned_by, "")


class TestEnsembleConfig(unittest.TestCase):
    """Test EnsembleConfig type."""

    def test_default_values(self):
        """Test default values."""
        config = EnsembleConfig()
        self.assertEqual(config.strategy, "confidence_weighted")
        self.assertEqual(config.min_providers, 2)
        self.assertEqual(config.confidence_threshold, 0.8)
        self.assertTrue(config.fallback_to_best)
        self.assertEqual(config.timeout, 30)
        self.assertEqual(config.preferred_providers, [])

    def test_custom_values(self):
        """Test custom values."""
        config = EnsembleConfig(
            strategy="majority_vote",
            min_providers=3,
            confidence_threshold=0.9,
            fallback_to_best=False,
            timeout=60,
            preferred_providers=["openai", "anthropic"],
        )
        self.assertEqual(config.strategy, "majority_vote")
        self.assertEqual(config.min_providers, 3)
        self.assertEqual(config.preferred_providers, ["openai", "anthropic"])

    def test_to_dict(self):
        """Test to_dict."""
        config = EnsembleConfig(
            strategy="best_of_n",
            min_providers=4,
            preferred_providers=["gemini"],
        )
        d = config.to_dict()
        self.assertEqual(d["strategy"], "best_of_n")
        self.assertEqual(d["min_providers"], 4)
        self.assertEqual(d["preferred_providers"], ["gemini"])
        self.assertTrue(d["fallback_to_best"])


if __name__ == "__main__":
    unittest.main()
