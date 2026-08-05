import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

import app.main as main_module
from app.database import get_db
from app.main import app


@pytest.fixture()
def client(monkeypatch):
    engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

    # main.py's lifespan/healthz use the module-global `engine` directly (not the
    # get_db dependency), so it must be patched too - otherwise they'd silently
    # run migrations against and connect to the real default sqlite file on disk
    # instead of this test's isolated in-memory database. Schema creation itself
    # happens via the lifespan's Alembic migration when TestClient starts up below,
    # not here - running Base.metadata.create_all first would conflict with it.
    monkeypatch.setattr(main_module, "engine", engine)

    def override_get_db():
        db = TestingSessionLocal()
        try:
            yield db
        finally:
            db.close()

    app.dependency_overrides[get_db] = override_get_db
    with TestClient(app) as test_client:
        yield test_client
    app.dependency_overrides.clear()


def register_and_login(client: TestClient, username: str, password: str = "s3cret-pass") -> str:
    client.post(
        "/auth/register",
        json={"username": username, "email": f"{username}@example.com", "password": password},
    )
    response = client.post(
        "/auth/login", data={"username": username, "password": password}
    )
    return response.json()["access_token"]
