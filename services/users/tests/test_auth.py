def test_register_creates_account(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "s3cret-pass"},
    )

    assert response.status_code == 201
    body = response.json()
    assert body["username"] == "alice"
    assert body["email"] == "alice@example.com"
    assert "password" not in body
    assert "hashed_password" not in body


def test_register_rejects_duplicate_username(client):
    payload = {"username": "alice", "email": "alice@example.com", "password": "s3cret-pass"}
    client.post("/auth/register", json=payload)

    response = client.post(
        "/auth/register",
        json={**payload, "email": "someone-else@example.com"},
    )

    assert response.status_code == 409


def test_login_returns_access_token_for_valid_credentials(client):
    client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "s3cret-pass"},
    )

    response = client.post(
        "/auth/login", data={"username": "alice", "password": "s3cret-pass"}
    )

    assert response.status_code == 200
    body = response.json()
    assert body["token_type"] == "bearer"
    assert body["access_token"]


def test_login_rejects_wrong_password(client):
    client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "s3cret-pass"},
    )

    response = client.post(
        "/auth/login", data={"username": "alice", "password": "wrong-password"}
    )

    assert response.status_code == 401


def test_login_rejects_unknown_username(client):
    response = client.post(
        "/auth/login", data={"username": "ghost", "password": "irrelevant"}
    )

    assert response.status_code == 401


def test_register_rejects_password_over_bcrypt_byte_limit(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "x" * 73},
    )

    assert response.status_code == 422


def test_register_rejects_too_short_password(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "short"},
    )

    assert response.status_code == 422


def test_register_rejects_password_over_bcrypt_byte_limit_with_multibyte_chars(client):
    # 24 "🙂" chars is 24 chars (well under a naive 72-char cap) but 96 bytes in UTF-8.
    response = client.post(
        "/auth/register",
        json={"username": "alice", "email": "alice@example.com", "password": "🙂" * 24},
    )

    assert response.status_code == 422


def test_register_rejects_reserved_username(client):
    response = client.post(
        "/auth/register",
        json={"username": "me", "email": "me@example.com", "password": "s3cret-pass"},
    )

    assert response.status_code == 422


def test_register_rejects_username_with_invalid_characters(client):
    response = client.post(
        "/auth/register",
        json={"username": "alice smith", "email": "alice@example.com", "password": "s3cret-pass"},
    )

    assert response.status_code == 422


def test_register_rejects_too_short_username(client):
    response = client.post(
        "/auth/register",
        json={"username": "ab", "email": "alice@example.com", "password": "s3cret-pass"},
    )

    assert response.status_code == 422
