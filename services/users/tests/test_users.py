from datetime import datetime, timezone

import app.models as models

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


def test_get_user_by_username_is_case_insensitive(client):
    register_and_login(client, "alice")

    response = client.get("/users/Alice")

    assert response.status_code == 200
    assert response.json()["username"] == "alice"


def test_get_user_by_username_does_not_leak_email(client):
    register_and_login(client, "alice")

    response = client.get("/users/alice")

    assert "email" not in response.json()


def test_followers_and_following_do_not_leak_email(client):
    alice_token = register_and_login(client, "alice")
    register_and_login(client, "bob")
    client.post("/users/bob/follow", headers={"Authorization": f"Bearer {alice_token}"})

    followers = client.get("/users/bob/followers").json()
    following = client.get("/users/alice/following").json()

    assert "email" not in followers[0]
    assert "email" not in following[0]


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


def test_followers_pagination(client):
    register_and_login(client, "popular")
    for name in ["fan-a", "fan-b", "fan-c"]:
        token = register_and_login(client, name)
        client.post("/users/popular/follow", headers={"Authorization": f"Bearer {token}"})

    first_page = client.get("/users/popular/followers", params={"limit": 2, "offset": 0}).json()
    second_page = client.get("/users/popular/followers", params={"limit": 2, "offset": 2}).json()

    assert [u["username"] for u in first_page] == ["fan-a", "fan-b"]
    assert [u["username"] for u in second_page] == ["fan-c"]


def test_followers_pagination_is_stable_when_timestamps_collide(client, monkeypatch):
    # Follow.created_at defaults to datetime.now(timezone.utc), so if several
    # follows land in the same instant (plausible in a burst), pagination must
    # still be deterministic via a tiebreaker rather than relying on timestamp order alone.
    frozen = datetime(2026, 1, 1, tzinfo=timezone.utc)

    class _FrozenDatetime:
        @staticmethod
        def now(tz=None):
            return frozen

    monkeypatch.setattr(models, "datetime", _FrozenDatetime)

    register_and_login(client, "popular")
    for name in ["fan-a", "fan-b", "fan-c"]:
        token = register_and_login(client, name)
        client.post("/users/popular/follow", headers={"Authorization": f"Bearer {token}"})

    first_page = client.get("/users/popular/followers", params={"limit": 2, "offset": 0}).json()
    second_page = client.get("/users/popular/followers", params={"limit": 2, "offset": 2}).json()

    assert [u["username"] for u in first_page] == ["fan-a", "fan-b"]
    assert [u["username"] for u in second_page] == ["fan-c"]
