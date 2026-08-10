import threading
import time
from collections.abc import Callable


class RateLimitExceeded(Exception):
    """Raised by RateLimiter.check when a key is over its attempt budget or locked out."""

    def __init__(self, retry_after: float):
        self.retry_after = retry_after
        super().__init__(f"rate limit exceeded, retry after {retry_after:.0f}s")


class _Bucket:
    __slots__ = ("count", "window_start", "locked_until")

    def __init__(self, window_start: float):
        self.count = 0
        self.window_start = window_start
        self.locked_until = 0.0


class RateLimiter:
    """Fixed-window attempt counter with a lockout once a key exceeds its budget.

    Each call to check() counts as one attempt. Once more than max_attempts
    attempts land within window_seconds, the key is locked out for
    lockout_seconds and every check() during that period raises
    RateLimitExceeded, independent of the window/count logic.
    """

    def __init__(
        self,
        max_attempts: int,
        window_seconds: float,
        lockout_seconds: float,
        now_fn: Callable[[], float] = time.monotonic,
    ):
        self._max_attempts = max_attempts
        self._window_seconds = window_seconds
        self._lockout_seconds = lockout_seconds
        self._now_fn = now_fn
        self._buckets: dict[str, _Bucket] = {}
        self._lock = threading.Lock()

    def check(self, key: str) -> None:
        now = self._now_fn()
        with self._lock:
            bucket = self._buckets.get(key)
            if bucket is None:
                bucket = _Bucket(window_start=now)
                self._buckets[key] = bucket

            if bucket.locked_until > now:
                raise RateLimitExceeded(retry_after=bucket.locked_until - now)

            if bucket.locked_until:
                # A previous lockout just expired: give the key a clean window
                # instead of counting from before the lockout, which would
                # otherwise re-trigger a lockout on the very next attempt.
                bucket.locked_until = 0.0
                bucket.window_start = now
                bucket.count = 0
            elif now - bucket.window_start >= self._window_seconds:
                bucket.window_start = now
                bucket.count = 0

            bucket.count += 1
            if bucket.count > self._max_attempts:
                bucket.locked_until = now + self._lockout_seconds
                raise RateLimitExceeded(retry_after=self._lockout_seconds)

    def reset(self, key: str) -> None:
        with self._lock:
            self._buckets.pop(key, None)

    def reset_all(self) -> None:
        with self._lock:
            self._buckets.clear()


def get_client_ip(request) -> str:
    """Best-effort client IP for rate-limit keying.

    Trusts X-Real-Ip / the last hop of X-Forwarded-For because Traefik (the only
    reverse proxy in front of this service, see infra/k3d/config.yaml) appends
    the peer address it saw to those headers rather than passing a
    client-supplied value through unchanged. Falls back to the socket peer
    address for direct connections (e.g. tests).
    """
    real_ip = request.headers.get("x-real-ip")
    if real_ip:
        return real_ip.strip()

    forwarded_for = request.headers.get("x-forwarded-for")
    if forwarded_for:
        return forwarded_for.split(",")[-1].strip()

    return request.client.host if request.client else "unknown"
