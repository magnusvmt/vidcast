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
    # Drop the old case-sensitive unique indexes first so the backfill below
    # doesn't fail on pre-existing case-variant duplicates (exactly the data
    # this migration exists to protect against). They are recreated below
    # after the functional indexes, for exact-match query performance.
    # Note: DROP INDEX / CREATE INDEX take table-level locks on Postgres, and
    # the whole sequence runs inside a single DDL transaction (see
    # app/migrations.py's upgrade_to_head), so the users table is effectively
    # locked for the duration. Acceptable at current pre-production scale; not
    # zero-downtime on a large live table.
    op.drop_index(op.f("ix_users_email"), table_name="users")
    op.drop_index(op.f("ix_users_username"), table_name="users")

    # Backfill any rows written before app-level lowercase normalization
    # (app/schemas.py's UserCreate validators) existed, so the unique indexes
    # below don't fail on pre-existing data.
    # Only rewrite rows that aren't already lowercase to avoid unnecessary
    # write amplification on a clean table.
    # Note: SQL lower() and Python str.lower() can disagree for non-ASCII
    # characters (e.g. Turkish I). Usernames are constrained to ASCII so this
    # doesn't apply there; for email, the app layer already normalizes on write
    # via UserCreate, so this DB-level constraint is defense-in-depth covering
    # the common Latin-script case.
    op.execute("UPDATE users SET email = lower(email) WHERE email <> lower(email)")
    op.execute("UPDATE users SET username = lower(username) WHERE username <> lower(username)")

    op.create_index("ix_users_email_lower", "users", [sa.text("lower(email)")], unique=True)
    op.create_index(
        "ix_users_username_lower", "users", [sa.text("lower(username)")], unique=True
    )

    # Recreate the plain unique indexes for exact-match query performance on
    # login/user lookups (functional indexes on lower() can't serve those).
    op.create_index(op.f("ix_users_email"), "users", ["email"], unique=True)
    op.create_index(op.f("ix_users_username"), "users", ["username"], unique=True)


def downgrade() -> None:
    # Note: the lowercase backfill in upgrade() is intentionally NOT reversed
    # here. Rows that were lowercased by upgrade() stay lowercased after a
    # downgrade — there is no clean way to restore their original case without
    # preserving pre-migration state, and keeping them lowercase is harmless
    # (app-level validation already normalizes new input).
    op.drop_index("ix_users_username_lower", table_name="users")
    op.drop_index("ix_users_email_lower", table_name="users")
