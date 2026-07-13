from datetime import datetime, timezone

from sqlalchemy import DateTime, ForeignKey, Index, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.database import Base


class User(Base):
    __tablename__ = "users"

    id: Mapped[int] = mapped_column(primary_key=True)
    username: Mapped[str] = mapped_column(String(32), unique=True, index=True)
    email: Mapped[str] = mapped_column(String(255), unique=True, index=True)
    hashed_password: Mapped[str] = mapped_column(String(255))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )


class Follow(Base):
    __tablename__ = "follows"
    __table_args__ = (
        # list_followers/list_following filter on one of these columns and sort by
        # created_at; a composite index lets that be satisfied without a separate sort.
        Index("ix_follows_followed_id_created_at", "followed_id", "created_at"),
        Index("ix_follows_follower_id_created_at", "follower_id", "created_at"),
    )

    follower_id: Mapped[int] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), primary_key=True
    )
    followed_id: Mapped[int] = mapped_column(
        ForeignKey("users.id", ondelete="CASCADE"), primary_key=True
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    follower: Mapped["User"] = relationship(foreign_keys=[follower_id], passive_deletes=True)
    followed: Mapped["User"] = relationship(foreign_keys=[followed_id], passive_deletes=True)
