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
