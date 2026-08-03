from alembic.autogenerate import compare_metadata
from alembic.runtime.migration import MigrationContext
from sqlalchemy import create_engine, inspect
from sqlalchemy.pool import StaticPool

from app.database import Base
from app.migrations import _SERVICE_ROOT, upgrade_to_head


def _fresh_engine():
    return create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )


def test_upgrade_to_head_creates_schema_matching_models():
    engine = _fresh_engine()

    upgrade_to_head(engine)

    with engine.connect() as connection:
        context = MigrationContext.configure(connection)
        diff = compare_metadata(context, Base.metadata)

    assert diff == []


def test_upgrade_to_head_stamps_alembic_version_table():
    # Confirms this went through Alembic (and is therefore upgradeable later)
    # rather than some other means of creating the same tables.
    engine = _fresh_engine()

    upgrade_to_head(engine)

    assert "alembic_version" in inspect(engine).get_table_names()


def test_upgrade_to_head_is_idempotent():
    # The users service runs this on every pod startup (see app.main.lifespan);
    # with multiple replicas or restarts it must be safe to run against a
    # database that's already at head.
    engine = _fresh_engine()

    upgrade_to_head(engine)
    upgrade_to_head(engine)

    assert set(inspect(engine).get_table_names()) >= {"users", "follows", "alembic_version"}


def test_initial_revision_has_no_down_revision():
    versions_dir = _SERVICE_ROOT / "alembic" / "versions"
    revision_files = [p for p in versions_dir.glob("*.py") if p.name != "__init__.py"]

    assert len(revision_files) == 1, "expected exactly one migration so far"
