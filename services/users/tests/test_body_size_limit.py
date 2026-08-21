import asyncio

import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from app.config import settings
from app.database import engine as prod_engine
from app.database import get_db
from app.migrations import upgrade_to_head

import app.main as main_module
from app.main import app

OVERSIZED = "x" * (settings.max_request_body_bytes + 1)


def test_register_rejects_oversized_body_before_validation(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": OVERSIZED},
    )
    assert response.status_code == 413


def test_login_rejects_oversized_body(client):
    response = client.post(
        "/auth/login",
        data={"username": "alice", "password": OVERSIZED},
    )
    assert response.status_code == 413


def test_register_still_accepts_normal_sized_request(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "s3cret-pass"},
    )
    assert response.status_code == 201


def _chunks(body, chunk_size=100):
    chunks = []
    offset = 0
    while offset < len(body):
        end = offset + chunk_size
        chunks.append({"type": "http.request", "body": body[offset:end], "more_body": True})
        offset = end
    if chunks:
        chunks[-1]["more_body"] = False
    else:
        chunks.append({"type": "http.request", "body": b"", "more_body": False})
    return chunks


@pytest.fixture(scope="module")
def asgi_app():
    test_engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )

    import app.database as db_module

    db_module.engine = test_engine
    main_module.engine = test_engine
    TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=test_engine)

    def override_get_db():
        db = TestingSessionLocal()
        try:
            yield db
        finally:
            db.close()

    app.dependency_overrides[get_db] = override_get_db
    upgrade_to_head(test_engine)
    yield app
    app.dependency_overrides.clear()
    db_module.engine = prod_engine
    main_module.engine = prod_engine


def _run_asgi(asgi_app, scope, chunks):
    response_status = None
    response_body = bytearray()

    async def _run():
        nonlocal response_status, response_body

        lifespan_scope = {"type": "lifespan", "asgi": {"version": "3.0"}}

        async def lifespan_receive():
            return {"type": "lifespan.startup"}

        async def lifespan_send(message):
            pass

        await asgi_app(lifespan_scope, lifespan_receive, lifespan_send)

        body_chunks = list(chunks)

        async def http_receive():
            if body_chunks:
                return body_chunks.pop(0)
            return {"type": "http.request", "body": b"", "more_body": False}

        async def http_send(message):
            nonlocal response_status, response_body
            if message["type"] == "http.response.start":
                response_status = message["status"]
            elif message["type"] == "http.response.body":
                response_body.extend(message.get("body", b""))

        await asgi_app(scope, http_receive, http_send)

    asyncio.run(_run())
    return response_status, bytes(response_body)


def _make_scope(path, method="POST", headers=None):
    return {
        "type": "http",
        "http_version": "1.1",
        "method": method,
        "scheme": "http",
        "path": path,
        "root_path": "",
        "headers": headers or [],
        "query_string": b"",
        "client": ("127.0.0.1", 50000),
        "server": ("127.0.0.1", 8000),
    }


def test_register_rejects_chunked_oversized_body_with_413(asgi_app):
    """A chunked body (no Content-Length) exceeding the limit returns 413
    through the real FastAPI app (exercises the streaming-counter path)."""
    scope = _make_scope("/auth/register")
    body = b'{"username":"alice","email":"a@example.com","password":"' + OVERSIZED.encode() + b'"}'
    status, _ = _run_asgi(asgi_app, scope, _chunks(body))
    assert status == 413


def test_login_rejects_chunked_oversized_body_with_413(asgi_app):
    """A chunked form-encoded body (no Content-Length) exceeding the limit
    returns 413 through the real FastAPI app (exercises the streaming-counter
    path for the form-parsing code path too)."""
    scope = _make_scope(
        "/auth/login",
        headers=[(b"content-type", b"application/x-www-form-urlencoded")],
    )
    body = b"username=alice&password=" + OVERSIZED.encode()
    status, _ = _run_asgi(asgi_app, scope, _chunks(body))
    assert status == 413