from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from app.database import Base
from app.models import Follow, User


def test_sqlite_fk_pragma_applies_to_any_engine():
    # The connect-event listener in app.database is bound to the SQLAlchemy
    # Engine *class*, not one specific instance, so any SQLite engine created
    # elsewhere - like this one, or the one tests/conftest.py builds for the
    # `client` fixture - gets PRAGMA foreign_keys=ON and therefore honors
    # Follow's ondelete="CASCADE" the same way Postgres does in prod.
    engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    Base.metadata.create_all(bind=engine)
    session = sessionmaker(bind=engine)()

    alice = User(username="alice", email="alice@example.com", hashed_password="x")
    bob = User(username="bob", email="bob@example.com", hashed_password="x")
    session.add_all([alice, bob])
    session.commit()

    session.add(Follow(follower_id=alice.id, followed_id=bob.id))
    session.commit()

    session.delete(alice)
    session.commit()

    assert session.query(Follow).count() == 0
