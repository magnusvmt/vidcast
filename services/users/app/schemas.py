from datetime import datetime

from pydantic import BaseModel, ConfigDict, EmailStr, Field, field_validator

# "me" would be permanently unreachable via GET /users/{username}, since the
# literal /users/me route is registered first.
RESERVED_USERNAMES = {"me"}


class UserCreate(BaseModel):
    username: str = Field(min_length=3, max_length=32, pattern=r"^[a-zA-Z0-9_-]+$")
    email: EmailStr
    # 72 chars max — matches bcrypt's 72-byte limit for ASCII input and catches
    # oversized passwords at the Field level before the byte-limit validator runs.
    password: str = Field(min_length=8, max_length=72)

    @field_validator("username")
    @classmethod
    def _reject_reserved_username(cls, value: str) -> str:
        if value.lower() in RESERVED_USERNAMES:
            raise ValueError(f"username {value!r} is reserved")
        return value

    @field_validator("username")
    @classmethod
    def _normalize_username(cls, value: str) -> str:
        # username is a unique lookup key just like email, so normalize it the
        # same way to prevent alice/Alice from registering as two accounts.
        return value.lower()

    @field_validator("password")
    @classmethod
    def _enforce_bcrypt_byte_limit(cls, value: str) -> str:
        # bcrypt's input limit is 72 *bytes*, not characters — a password within
        # a character-count limit can still overflow once UTF-8 encoded.
        if len(value.encode("utf-8")) > 72:
            raise ValueError("password must be at most 72 bytes")
        return value

    @field_validator("email")
    @classmethod
    def _normalize_email(cls, value: str) -> str:
        # most providers treat email addresses case-insensitively; normalize
        # to lowercase so the DB's unique constraint actually prevents
        # duplicate accounts like alice@example.com / Alice@Example.com.
        return value.lower()


class PublicUserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    username: str
    created_at: datetime


class UserOut(PublicUserOut):
    email: EmailStr


class Token(BaseModel):
    access_token: str
    token_type: str = "bearer"
