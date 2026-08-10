import pytest

from app.rate_limit import RateLimiter, RateLimitExceeded, get_client_ip


def make_limiter(max_attempts=3, window_seconds=60.0, lockout_seconds=120.0, now=0.0):
    clock = {"t": now}
    limiter = RateLimiter(
        max_attempts=max_attempts,
        window_seconds=window_seconds,
        lockout_seconds=lockout_seconds,
        now_fn=lambda: clock["t"],
    )
    return limiter, clock


def test_allows_up_to_max_attempts_within_window():
    limiter, _clock = make_limiter(max_attempts=3)

    for _ in range(3):
        limiter.check("1.2.3.4")


def test_raises_once_max_attempts_exceeded():
    limiter, _clock = make_limiter(max_attempts=3)
    for _ in range(3):
        limiter.check("1.2.3.4")

    with pytest.raises(RateLimitExceeded):
        limiter.check("1.2.3.4")


def test_lockout_blocks_further_attempts_until_it_expires():
    limiter, clock = make_limiter(max_attempts=1, window_seconds=60.0, lockout_seconds=30.0)
    limiter.check("1.2.3.4")

    with pytest.raises(RateLimitExceeded) as exc_info:
        limiter.check("1.2.3.4")
    assert exc_info.value.retry_after == pytest.approx(30.0)

    clock["t"] += 29.9
    with pytest.raises(RateLimitExceeded):
        limiter.check("1.2.3.4")

    clock["t"] += 0.2
    limiter.check("1.2.3.4")


def test_window_resets_count_after_it_elapses_without_exceeding():
    limiter, clock = make_limiter(max_attempts=2, window_seconds=10.0, lockout_seconds=60.0)
    limiter.check("1.2.3.4")

    clock["t"] += 11.0
    limiter.check("1.2.3.4")
    limiter.check("1.2.3.4")


def test_keys_are_tracked_independently():
    limiter, _clock = make_limiter(max_attempts=1)
    limiter.check("1.2.3.4")

    limiter.check("5.6.7.8")


def test_reset_clears_a_keys_state():
    limiter, _clock = make_limiter(max_attempts=1)
    limiter.check("1.2.3.4")
    limiter.reset("1.2.3.4")

    limiter.check("1.2.3.4")


def test_reset_all_clears_every_key():
    limiter, _clock = make_limiter(max_attempts=1)
    limiter.check("1.2.3.4")
    limiter.check("5.6.7.8")
    limiter.reset_all()

    limiter.check("1.2.3.4")
    limiter.check("5.6.7.8")


class _FakeClient:
    def __init__(self, host):
        self.host = host


class _FakeRequest:
    def __init__(self, headers=None, client_host="203.0.113.5"):
        self.headers = headers or {}
        self.client = _FakeClient(client_host) if client_host is not None else None


def test_get_client_ip_prefers_x_real_ip_header():
    request = _FakeRequest(headers={"x-real-ip": "9.9.9.9"}, client_host="203.0.113.5")

    assert get_client_ip(request) == "9.9.9.9"


def test_get_client_ip_falls_back_to_last_x_forwarded_for_hop():
    request = _FakeRequest(
        headers={"x-forwarded-for": "1.1.1.1, 2.2.2.2"}, client_host="203.0.113.5"
    )

    assert get_client_ip(request) == "2.2.2.2"


def test_get_client_ip_falls_back_to_request_client_host():
    request = _FakeRequest(headers={}, client_host="203.0.113.5")

    assert get_client_ip(request) == "203.0.113.5"


def test_get_client_ip_falls_back_to_unknown_when_no_client():
    request = _FakeRequest(headers={}, client_host=None)

    assert get_client_ip(request) == "unknown"


def test_evict_stale_removes_single_touch_entries_after_window_expires():
    window_seconds = 10.0
    limiter, clock = make_limiter(
        max_attempts=5,
        window_seconds=window_seconds,
        lockout_seconds=30.0,
        now=0.0,
    )
    limiter._eviction_sample_size = 2

    limiter.check("user-a")
    limiter.check("user-b")
    limiter.check("user-c")

    assert len(limiter._buckets) == 3

    clock["t"] += window_seconds * 2 + 1

    limiter.check("user-other")

    assert "user-a" not in limiter._buckets
    assert "user-b" not in limiter._buckets
    assert "user-c" not in limiter._buckets
