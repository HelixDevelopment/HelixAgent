"""
Custom exceptions for SuperAgent SDK.
"""

from typing import Optional, Dict, Any


class SuperAgentError(Exception):
    """Base exception for SuperAgent SDK."""

    def __init__(
        self,
        message: str,
        status_code: Optional[int] = None,
        response: Optional[Dict[str, Any]] = None,
    ):
        super().__init__(message)
        self.message = message
        self.status_code = status_code
        self.response = response

    def __str__(self) -> str:
        if self.status_code:
            return f"[{self.status_code}] {self.message}"
        return self.message


class AuthenticationError(SuperAgentError):
    """Raised when authentication fails (401)."""
    pass


class RateLimitError(SuperAgentError):
    """Raised when rate limit is exceeded (429)."""

    def __init__(
        self,
        message: str,
        retry_after: Optional[int] = None,
        **kwargs,
    ):
        super().__init__(message, **kwargs)
        self.retry_after = retry_after


class APIError(SuperAgentError):
    """Raised for general API errors (4xx, 5xx)."""
    pass


class ConnectionError(SuperAgentError):
    """Raised when connection to the API fails."""
    pass


class ValidationError(SuperAgentError):
    """Raised when request validation fails."""
    pass


class TimeoutError(SuperAgentError):
    """Raised when a request times out."""
    pass


def raise_for_status(status_code: int, response_data: Dict[str, Any]) -> None:
    """Raise appropriate exception based on status code."""
    error_message = response_data.get("error", {})
    if isinstance(error_message, dict):
        message = error_message.get("message", "Unknown error")
    else:
        message = str(error_message) if error_message else "Unknown error"

    if status_code == 401:
        raise AuthenticationError(
            message=message,
            status_code=status_code,
            response=response_data,
        )
    elif status_code == 429:
        retry_after = response_data.get("retry_after")
        raise RateLimitError(
            message=message,
            status_code=status_code,
            response=response_data,
            retry_after=retry_after,
        )
    elif 400 <= status_code < 500:
        raise ValidationError(
            message=message,
            status_code=status_code,
            response=response_data,
        )
    elif status_code >= 500:
        raise APIError(
            message=message,
            status_code=status_code,
            response=response_data,
        )
