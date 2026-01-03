"""Tests for SuperAgent Verifier SDK exceptions."""

import unittest

from superagent_verifier.exceptions import (
    VerifierError,
    APIError,
    ValidationError,
    AuthenticationError,
    NotFoundError,
    TimeoutError,
    RateLimitError,
)


class TestVerifierError(unittest.TestCase):
    """Test VerifierError base exception."""

    def test_basic_error(self):
        """Test basic error."""
        error = VerifierError("Something went wrong")
        self.assertEqual(str(error), "Something went wrong")

    def test_inheritance(self):
        """Test inheritance from Exception."""
        error = VerifierError("Error")
        self.assertIsInstance(error, Exception)


class TestAPIError(unittest.TestCase):
    """Test APIError exception."""

    def test_basic_error(self):
        """Test basic API error."""
        error = APIError("API request failed")
        self.assertEqual(str(error), "API request failed")
        self.assertIsNone(error.status_code)
        self.assertIsNone(error.response)

    def test_with_status_code(self):
        """Test with status code."""
        error = APIError("Server error", status_code=500)
        self.assertEqual(error.status_code, 500)

    def test_with_response(self):
        """Test with response."""
        response = {"error": "Internal error", "details": "Stack trace"}
        error = APIError("Server error", status_code=500, response=response)
        self.assertEqual(error.response, response)

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = APIError("Error")
        self.assertIsInstance(error, VerifierError)


class TestValidationError(unittest.TestCase):
    """Test ValidationError exception."""

    def test_basic_error(self):
        """Test basic validation error."""
        error = ValidationError("Invalid model_id")
        self.assertEqual(str(error), "Invalid model_id")

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = ValidationError("Error")
        self.assertIsInstance(error, VerifierError)


class TestAuthenticationError(unittest.TestCase):
    """Test AuthenticationError exception."""

    def test_basic_error(self):
        """Test basic authentication error."""
        error = AuthenticationError("Invalid API key")
        self.assertEqual(str(error), "Invalid API key")

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = AuthenticationError("Error")
        self.assertIsInstance(error, VerifierError)


class TestNotFoundError(unittest.TestCase):
    """Test NotFoundError exception."""

    def test_basic_error(self):
        """Test basic not found error."""
        error = NotFoundError("Model not found")
        self.assertEqual(str(error), "Model not found")

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = NotFoundError("Error")
        self.assertIsInstance(error, VerifierError)


class TestTimeoutError(unittest.TestCase):
    """Test TimeoutError exception."""

    def test_basic_error(self):
        """Test basic timeout error."""
        error = TimeoutError("Request timed out")
        self.assertEqual(str(error), "Request timed out")

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = TimeoutError("Error")
        self.assertIsInstance(error, VerifierError)


class TestRateLimitError(unittest.TestCase):
    """Test RateLimitError exception."""

    def test_basic_error(self):
        """Test basic rate limit error."""
        error = RateLimitError("Rate limit exceeded")
        self.assertEqual(str(error), "Rate limit exceeded")
        self.assertIsNone(error.retry_after)

    def test_with_retry_after(self):
        """Test with retry_after."""
        error = RateLimitError("Rate limit exceeded", retry_after=30)
        self.assertEqual(error.retry_after, 30)

    def test_inheritance(self):
        """Test inheritance from VerifierError."""
        error = RateLimitError("Error")
        self.assertIsInstance(error, VerifierError)


if __name__ == "__main__":
    unittest.main()
