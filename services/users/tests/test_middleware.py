import asyncio

from app.middleware import MaxBodySizeMiddleware


class RecordingApp:
    """A minimal ASGI app that drains the body via `receive` then responds.

    Mirrors what Starlette/FastAPI do internally: keep calling `receive`
    until `more_body` is False, accumulating whatever bytes arrive.
    """

    def __init__(self):
        self.received = b""
        self.called = False

    async def __call__(self, scope, receive, send):
        self.called = True
        while True:
            message = await receive()
            self.received += message.get("body", b"")
            if not message.get("more_body", False):
                break
        await send({"type": "http.response.start", "status": 200, "headers": []})
        await send({"type": "http.response.body", "body": b"ok"})


def make_scope(headers=None):
    return {
        "type": "http",
        "method": "POST",
        "path": "/",
        "headers": headers or [],
    }


def chunked_receive(chunks):
    """Build an ASGI `receive` callable that yields the given (body, more_body) chunks.

    No Content-Length header is implied - this simulates chunked transfer
    encoding, where the only way to know the body is oversized is to count
    bytes as they stream in.
    """
    remaining = list(chunks)

    async def receive():
        if not remaining:
            return {"type": "http.request", "body": b"", "more_body": False}
        return remaining.pop(0)

    return receive


async def collect_send():
    messages = []

    async def send(message):
        messages.append(message)

    return messages, send


def run(coro):
    return asyncio.run(coro)


def test_allows_body_within_limit():
    app = RecordingApp()
    middleware = MaxBodySizeMiddleware(app, max_body_size=10)
    receive = chunked_receive([{"type": "http.request", "body": b"12345", "more_body": False}])
    messages, send = run(collect_send())

    run(middleware(make_scope(), receive, send))

    assert app.called
    assert app.received == b"12345"
    assert messages[0] == {"type": "http.response.start", "status": 200, "headers": []}


def test_rejects_body_over_limit_via_content_length_without_reading_body():
    app = RecordingApp()
    middleware = MaxBodySizeMiddleware(app, max_body_size=10)
    scope = make_scope(headers=[(b"content-length", b"1000")])

    async def receive():
        raise AssertionError("body should never be read when Content-Length already exceeds the limit")

    messages, send = run(collect_send())

    run(middleware(scope, receive, send))

    assert not app.called
    assert messages[0]["type"] == "http.response.start"
    assert messages[0]["status"] == 413


def test_rejects_streamed_body_over_limit_without_lying_content_length():
    app = RecordingApp()
    middleware = MaxBodySizeMiddleware(app, max_body_size=10)
    # No content-length header at all (as with chunked transfer encoding):
    # the only way to catch this is counting bytes as they stream in.
    receive = chunked_receive(
        [
            {"type": "http.request", "body": b"12345", "more_body": True},
            {"type": "http.request", "body": b"67890", "more_body": True},
            {"type": "http.request", "body": b"extra-bytes-that-push-it-over", "more_body": False},
        ]
    )
    messages, send = run(collect_send())

    run(middleware(make_scope(), receive, send))

    assert messages[0]["type"] == "http.response.start"
    assert messages[0]["status"] == 413
    # The middleware must stop before the app ever sees the full oversized
    # body - the whole point is to avoid buffering it in memory.
    assert len(app.received) <= 10


def test_rejects_body_over_limit_with_understated_content_length():
    app = RecordingApp()
    middleware = MaxBodySizeMiddleware(app, max_body_size=10)
    # Content-Length claims to be within the limit, but more bytes actually
    # arrive - the streaming counter must still catch it.
    scope = make_scope(headers=[(b"content-length", b"5")])
    receive = chunked_receive(
        [
            {"type": "http.request", "body": b"12345", "more_body": True},
            {"type": "http.request", "body": b"67890extra", "more_body": False},
        ]
    )
    messages, send = run(collect_send())

    run(middleware(scope, receive, send))

    assert messages[0]["type"] == "http.response.start"
    assert messages[0]["status"] == 413


def test_swallowed_exception_on_oversized_body_is_logged(caplog):
    class RaisingApp:
        async def __call__(self, scope, receive, send):
            while True:
                message = await receive()
                if not message.get("more_body", False):
                    break
            raise RuntimeError("unrelated bug in the wrapped app")

    middleware = MaxBodySizeMiddleware(RaisingApp(), max_body_size=10)
    receive = chunked_receive(
        [{"type": "http.request", "body": b"way-too-many-bytes", "more_body": False}]
    )
    messages, send = run(collect_send())

    with caplog.at_level("ERROR"):
        run(middleware(make_scope(), receive, send))

    assert messages[0]["type"] == "http.response.start"
    assert messages[0]["status"] == 413
    assert any("unrelated error" in record.message for record in caplog.records)


def test_non_http_scope_passes_through_untouched():
    app = RecordingApp()
    middleware = MaxBodySizeMiddleware(app, max_body_size=10)
    scope = {"type": "lifespan"}

    async def receive():
        return {"type": "lifespan.startup"}

    sent = []

    async def send(message):
        sent.append(message)

    run(middleware(scope, receive, send))

    assert app.called
