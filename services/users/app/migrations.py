import hashlib
import time
from pathlib import Path

from alembic import command
from alembic.config import Config
from sqlalchemy import text
from sqlalchemy.engine import Engine

_ADVISORY_LOCK_KEY = int(hashlib.sha256(b"users.alembic.upgrade").hexdigest(), 16) & 0x7FFFFFFFFFFFFFFF
_ADVISORY_LOCK_TIMEOUT_S = 30
_ADVISORY_LOCK_RETRY_S = 0.2

_SERVICE_ROOT = Path(__file__).resolve().parent.parent
_ALEMBIC_INI = _SERVICE_ROOT / "alembic.ini"


def _acquire_advisory_lock(connection) -> None:
    """Acquire the migration advisory lock, waiting up to _ADVISORY_LOCK_TIMEOUT_S.

    Polls pg_try_advisory_lock rather than calling pg_advisory_lock so a lock
    stuck from a crashed deployment surfaces as a clear startup error instead
    of blocking every subsequent pod's lifespan forever.
    """
    deadline = time.monotonic() + _ADVISORY_LOCK_TIMEOUT_S
    while True:
        if connection.execute(
            text("SELECT pg_try_advisory_lock(:lock_key)"), {"lock_key": _ADVISORY_LOCK_KEY}
        ).scalar():
            return
        if time.monotonic() >= deadline:
            raise RuntimeError(
                f"timed out after {_ADVISORY_LOCK_TIMEOUT_S}s waiting for the "
                f"migration advisory lock {_ADVISORY_LOCK_KEY}"
            )
        time.sleep(_ADVISORY_LOCK_RETRY_S)


def upgrade_to_head(bind: Engine) -> None:
    """Run Alembic migrations up to head against the given engine.

    Runs against the caller's own engine/connection (via Alembic's
    `attributes["connection"]` hook) instead of letting alembic/env.py build
    its own from settings, so tests can point this at an isolated in-memory
    engine the same way they already do for the app's normal request-time
    `engine` (see tests/conftest.py).

    Acquires a Postgres advisory lock around the migration so that multiple
    pods starting simultaneously (replicaCount > 1) don't race on the same DDL.
    The lock acquisition is bounded by _ADVISORY_LOCK_TIMEOUT_S so a stuck
    lock from a crashed deployment surfaces as a clear startup error rather
    than hanging every subsequent pod's lifespan forever. SQLite has no
    advisory locks, so the guard is a no-op there.
    """
    config = Config(str(_ALEMBIC_INI))
    # Set explicitly rather than relying on alembic.ini's relative path, whose
    # resolution depends on the process's current working directory.
    config.set_main_option("script_location", str(_SERVICE_ROOT / "alembic"))
    with bind.connect() as connection:
        if connection.dialect.name == "postgresql":
            _acquire_advisory_lock(connection)
        try:
            config.attributes["connection"] = connection
            command.upgrade(config, "head")
            connection.commit()
        finally:
            if connection.dialect.name == "postgresql":
                # A failed command.upgrade leaves Postgres with an aborted
                # transaction; rolling back first lets the unlock run (so the
                # session-level lock is actually released) instead of failing
                # and masking the original migration error.
                connection.rollback()
                connection.execute(text("SELECT pg_advisory_unlock(:lock_key)"), {"lock_key": _ADVISORY_LOCK_KEY})