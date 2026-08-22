import logging

from starlette.responses import JSONResponse
from starlette.types import ASGIApp, Message, Receive, Scope, Send

logger = logging.getLogger(__name__)


class MaxBodySizeMiddleware:
    """Rejects request bodies larger than max_body_size with a 413.

    Implemented as raw ASGI (not BaseHTTPMiddleware, which buffers the whole
    body into memory before your code sees it) so an oversized body is
    rejected as it streams in, before it's ever fully buffered:

    - If Content-Length is present and already over the limit, reject
      immediately without reading any body bytes.
    - Otherwise, count bytes as they arrive via the wrapped `receive`
      callable (covers chunked transfer encoding and a Content-Length
      header that understates the real body size) and abort as soon as the
      running total crosses the limit.
    """

    def __init__(self, app: ASGIApp, max_body_size: int) -> None:
        self.app = app
        self.max_body_size = max_body_size

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        content_length = _content_length(scope)
        if content_length is not None and content_length > self.max_body_size:
            await _too_large_response(scope, receive, send)
            return

        received = 0
        exceeded = False

        async def guarded_receive() -> Message:
            nonlocal received, exceeded
            if exceeded:
                return {"type": "http.request", "body": b"", "more_body": False}
            message = await receive()
            if message["type"] == "http.request":
                received += len(message.get("body", b""))
                if received > self.max_body_size:
                    exceeded = True
                    return {"type": "http.request", "body": b"", "more_body": False}
            return message

        async def intercepted_send(message: Message) -> None:
            nonlocal exceeded
            if exceeded and message["type"] in (
                "http.response.start",
                "http.response.body",
            ):
                return
            await send(message)

        try:
            await self.app(scope, guarded_receive, intercepted_send)
        except Exception:
            if not exceeded:
                raise
            logger.exception(
                "inner app raised while handling an oversized request body; "
                "responding 413 but logging in case this masks an unrelated error"
            )
        if exceeded:
            await _too_large_response(scope, receive, send)


def _content_length(scope: Scope) -> int | None:
    for name, value in scope.get("headers", []):
        if name == b"content-length":
            try:
                return int(value)
            except ValueError:
                return None
    return None


async def _too_large_response(scope: Scope, receive: Receive, send: Send) -> None:
    response = JSONResponse({"detail": "request body too large"}, status_code=413)
    await response(scope, receive, send)
