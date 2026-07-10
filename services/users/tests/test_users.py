from tests.conftest import register_and_login


def test_me_requires_authentication(client):
    response = client.get("/users/me")

    assert response.status_code == 401


def test_me_returns_current_user(client):
    token = register_and_login(client, "alice")

    response = client.get("/users/me", headers={"Authorization": f"Bearer {token}"})

    assert response.status_code == 200
    assert response.json()["username"] == "alice"


def test_get_user_by_username(client):
    register_and_login(client, "alice")

    response = client.get("/users/alice")

    assert response.status_code == 200
    assert response.json()["username"] == "alice"


def test_get_unknown_user_returns_404(client):
    response = client.get("/users/ghost")

    assert response.status_code == 404


def test_follow_and_list_followers(client):
    alice_token = register_and_login(client, "alice")
    register_and_login(client, "bob")

    response = client.post(
        "/users/bob/follow", headers={"Authorization": f"Bearer {alice_token}"}
    )
    assert response.status_code == 204

    followers = client.get("/users/bob/followers").json()
    assert [u["username"] for u in followers] == ["alice"]

    following = client.get("/users/alice/following").json()
    assert [u["username"] for u in following] == ["bob"]


def test_following_twice_is_idempotent(client):
    alice_token = register_and_login(client, "alice")
    register_and_login(client, "bob")
    headers = {"Authorization": f"Bearer {alice_token}"}

    first = client.post("/users/bob/follow", headers=headers)
    second = client.post("/users/bob/follow", headers=headers)

    assert first.status_code == 204
    assert second.status_code == 204
    assert [u["username"] for u in client.get("/users/bob/followers").json()] == ["alice"]


def test_cannot_follow_self(client):
    alice_token = register_and_login(client, "alice")

    response = client.post(
        "/users/alice/follow", headers={"Authorization": f"Bearer {alice_token}"}
    )

    assert response.status_code == 400


def test_unfollow_removes_relationship(client):
    alice_token = register_and_login(client, "alice")
    register_and_login(client, "bob")
    headers = {"Authorization": f"Bearer {alice_token}"}
    client.post("/users/bob/follow", headers=headers)

    response = client.delete("/users/bob/follow", headers=headers)

    assert response.status_code == 204
    assert client.get("/users/bob/followers").json() == []
