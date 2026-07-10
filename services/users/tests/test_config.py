import pytest
from pydantic import ValidationError

from app.config import Settings


def test_resolved_database_url_falls_back_to_database_url_without_host():
    settings = Settings(database_url="sqlite:////tmp/x.db")

    assert settings.resolved_database_url == "sqlite:////tmp/x.db"


def test_resolved_database_url_builds_psycopg_url_from_discrete_fields():
    settings = Settings(
        db_host="users-db-rw",
        db_port=5432,
        db_name="app",
        db_user="app",
        db_password="s3cret",
    )

    assert (
        settings.resolved_database_url
        == "postgresql+psycopg://app:s3cret@users-db-rw:5432/app"
    )


def test_default_jwt_secret_is_fine_in_development():
    settings = Settings(environment="development")

    assert settings.jwt_secret == "dev-secret-do-not-use-in-production"


def test_default_jwt_secret_is_rejected_outside_development():
    with pytest.raises(ValidationError):
        Settings(environment="production")


def test_resolved_database_url_escapes_reserved_characters_in_credentials():
    settings = Settings(
        db_host="users-db-rw",
        db_port=5432,
        db_name="app",
        db_user="a@user",
        db_password="p@ss:word/1",
    )

    assert settings.resolved_database_url == (
        "postgresql+psycopg://a%40user:p%40ss%3Aword%2F1@users-db-rw:5432/app"
    )


def test_rejects_db_host_without_the_other_discrete_db_fields():
    with pytest.raises(ValidationError):
        Settings(db_host="users-db-rw")
