from app.config import settings

OVERSIZED = "x" * (settings.max_request_body_bytes + 1)


def test_register_rejects_oversized_body_before_validation(client):
    # A password this large would fail UserCreate's max_length=72 validation
    # anyway, but the point is that the body-size middleware rejects it
    # before FastAPI ever buffers/parses the payload to run that validation.
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
