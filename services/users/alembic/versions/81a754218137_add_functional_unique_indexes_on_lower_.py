"""add functional unique indexes on lower email and username

Revision ID: 81a754218137
Revises: 364ed08b6200
Create Date: 2026-08-15 04:25:05.724816

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '81a754218137'
down_revision: Union[str, None] = '364ed08b6200'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Backfill any rows written before app-level lowercase normalization
    # (app/schemas.py's UserCreate validators) existed, so the unique indexes
    # below don't fail on pre-existing data.
    op.execute("UPDATE users SET email = lower(email)")
    op.execute("UPDATE users SET username = lower(username)")
    op.create_index("ix_users_email_lower", "users", [sa.text("lower(email)")], unique=True)
    op.create_index(
        "ix_users_username_lower", "users", [sa.text("lower(username)")], unique=True
    )


def downgrade() -> None:
    op.drop_index("ix_users_username_lower", table_name="users")
    op.drop_index("ix_users_email_lower", table_name="users")
