import pytest
from alembic.autogenerate import compare_metadata
from alembic.runtime.migration import MigrationContext
from sqlalchemy import create_engine, inspect
from sqlalchemy.pool import StaticPool

from app import migrations as migrations_module
from app.database import Base
from app.migrations import upgrade_to_head


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


class _FakeDialect:
    name = "postgresql"


class _FakeResult:
    def __init__(self, value=True):
        self._value = value

    def scalar(self):
        return self._value


class _FakeConnection:
    dialect = _FakeDialect()

    def __init__(self, try_lock_result=True):
        self._try_lock_result = try_lock_result
        self.executed = []
        self.committed = False
        self.rolled_back = False

    def __enter__(self):
        return self

    def __exit__(self, *args):
        pass

    def execute(self, statement, params=None):
        self.executed.append(statement)
        if "pg_try_advisory_lock" in str(statement):
            return _FakeResult(self._try_lock_result)
        return _FakeResult(True)

    def commit(self):
        self.committed = True

    def rollback(self):
        self.rolled_back = True


class _FakeEngine:
    def __init__(self, connection):
        self._connection = connection

    def connect(self):
        return self._connection


def test_upgrade_to_head_rolls_back_before_unlock_when_upgrade_fails(monkeypatch):
    """Verifies that a failed command.upgrade on Postgres does not mask the
    original error — rollback clears the aborted transaction, unlock runs,
    and the real migration error propagates."""
    connection = _FakeConnection()
    engine = _FakeEngine(connection)

    def boom(config, revision):
        raise RuntimeError("migration failed")

    monkeypatch.setattr(migrations_module.command, "upgrade", boom)

    with pytest.raises(RuntimeError, match="migration failed"):
        upgrade_to_head(engine)

    assert connection.rolled_back
    assert any("pg_advisory_unlock" in str(s) for s in connection.executed)


def test_upgrade_to_head_lock_acquisition_times_out(monkeypatch):
    """Verifies that the advisory lock acquisition raises a clear error instead
    of hanging forever when the lock is held by another session."""
    connection = _FakeConnection(try_lock_result=False)
    engine = _FakeEngine(connection)

    monkeypatch.setattr(migrations_module, "_ADVISORY_LOCK_TIMEOUT_S", 0)
    monkeypatch.setattr(migrations_module.time, "sleep", lambda _s: None)

    with pytest.raises(RuntimeError, match="timed out"):
        upgrade_to_head(engine)

    assert not connection.committed
