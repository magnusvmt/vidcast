from collections.abc import Generator

from sqlalchemy import create_engine, event
from sqlalchemy.engine import Engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from app.config import settings

database_url = settings.resolved_database_url
connect_args = {"check_same_thread": False} if database_url.startswith("sqlite") else {}
engine = create_engine(
    database_url,
    connect_args=connect_args,
    # CloudNativePG restarts/switches over the primary independently of this
    # service, which can silently drop pooled connections; pre_ping catches
    # that before a query hits it, and recycle avoids reusing a connection
    # long enough for it to go stale in the first place.
    pool_pre_ping=True,
    pool_recycle=1800,
)

# Enable FK enforcement for SQLite (ondelete="CASCADE" doesn't work otherwise).
# Bound to the Engine class (dialect-filtered) rather than the module-global
# `engine` instance so it also applies to engines tests construct themselves,
# e.g. services/users/tests/conftest.py's in-memory SQLite engine.
@event.listens_for(Engine, "connect")
def set_sqlite_pragma(dbapi_conn, connection_record):
    if dbapi_conn.__class__.__module__.startswith("sqlite3"):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA foreign_keys=ON")
        cursor.close()


SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


class Base(DeclarativeBase):
    pass


def get_db() -> Generator[Session, None, None]:
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
