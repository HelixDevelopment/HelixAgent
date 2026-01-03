"""
Exceptions for the SuperAgent Verifier SDK.
"""


class VerifierError(Exception):
    """Base exception for verifier errors."""
    pass


class APIError(VerifierError):
    """Exception for API errors."""

    def __init__(self, message: str, status_code: int = None, response: dict = None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response


class ValidationError(VerifierError):
    """Exception for validation errors."""
    pass


class AuthenticationError(VerifierError):
    """Exception for authentication errors."""
    pass


class NotFoundError(VerifierError):
    """Exception for not found errors."""
    pass


class TimeoutError(VerifierError):
    """Exception for timeout errors."""
    pass


class RateLimitError(VerifierError):
    """Exception for rate limit errors."""

    def __init__(self, message: str, retry_after: int = None):
        super().__init__(message)
        self.retry_after = retry_after
