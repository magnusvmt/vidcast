from pathlib import Path

from alembic import command
from alembic.config import Config
from sqlalchemy.engine import Engine

_SERVICE_ROOT = Path(__file__).resolve().parent.parent
_ALEMBIC_INI = _SERVICE_ROOT / "alembic.ini"


def upgrade_to_head(bind: Engine) -> None:
    """Run Alembic migrations up to head against the given engine.

    Runs against the caller's own engine/connection (via Alembic's
    `attributes["connection"]` hook) instead of letting alembic/env.py build
    its own from settings, so tests can point this at an isolated in-memory
    engine the same way they already do for the app's normal request-time
    `engine` (see tests/conftest.py).
    """
    config = Config(str(_ALEMBIC_INI))
    # Set explicitly rather than relying on alembic.ini's relative path, whose
    # resolution depends on the process's current working directory.
    config.set_main_option("script_location", str(_SERVICE_ROOT / "alembic"))
    with bind.connect() as connection:
        config.attributes["connection"] = connection
        command.upgrade(config, "head")
