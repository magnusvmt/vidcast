import hashlib
from pathlib import Path

from alembic import command
from alembic.config import Config
from sqlalchemy import text
from sqlalchemy.engine import Engine

_ADVISORY_LOCK_KEY = int(hashlib.sha256(b"users.alembic.upgrade").hexdigest(), 16) & 0x7FFFFFFFFFFFFFFF

_SERVICE_ROOT = Path(__file__).resolve().parent.parent
_ALEMBIC_INI = _SERVICE_ROOT / "alembic.ini"


def upgrade_to_head(bind: Engine) -> None:
    """Run Alembic migrations up to head against the given engine.

    Runs against the caller's own engine/connection (via Alembic's
    `attributes["connection"]` hook) instead of letting alembic/env.py build
    its own from settings, so tests can point this at an isolated in-memory
    engine the same way they already do for the app's normal request-time
    `engine` (see tests/conftest.py).

    Acquires a Postgres advisory lock (pg_advisory_lock) around the migration
    so that multiple pods starting simultaneously (replicaCount > 1) don't race
    on the same DDL. The lock is session-level, so it persists across any
    transaction boundaries within command.upgrade. SQLite has no advisory
    locks, so the guard is a no-op there.
    """
    config = Config(str(_ALEMBIC_INI))
    # Set explicitly rather than relying on alembic.ini's relative path, whose
    # resolution depends on the process's current working directory.
    config.set_main_option("script_location", str(_SERVICE_ROOT / "alembic"))
    with bind.connect() as connection:
        if connection.dialect.name == "postgresql":
            connection.execute(text("SELECT pg_advisory_lock(:lock_key)"), {"lock_key": _ADVISORY_LOCK_KEY})
        try:
            config.attributes["connection"] = connection
            command.upgrade(config, "head")
            connection.commit()
        finally:
            if connection.dialect.name == "postgresql":
                connection.execute(text("SELECT pg_advisory_unlock(:lock_key)"), {"lock_key": _ADVISORY_LOCK_KEY})
