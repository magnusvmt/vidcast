import math

from fastapi import APIRouter, Depends, HTTPException, Request, status
from fastapi.security import OAuth2PasswordRequestForm
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.config import settings
from app.database import get_db
from app.models import User
from app.rate_limit import RateLimiter, RateLimitExceeded, get_client_ip
from app.schemas import Token, UserCreate, UserOut
from app.security import DUMMY_PASSWORD_HASH, create_access_token, hash_password, verify_password

router = APIRouter(prefix="/auth", tags=["auth"])

# Module-level singletons so attempt counts are shared across requests within a
# process. State is per-pod and in-memory (not shared across replicas or
# persisted across restarts) - an accepted v1 trade-off for a fallback
# in-app limiter; see issue #56 for the gateway-level alternative.
register_ip_limiter = RateLimiter(
    max_attempts=settings.register_rate_limit_max_attempts,
    window_seconds=settings.register_rate_limit_window_seconds,
    lockout_seconds=settings.register_rate_limit_lockout_seconds,
)
login_ip_limiter = RateLimiter(
    max_attempts=settings.login_rate_limit_max_attempts,
    window_seconds=settings.login_rate_limit_window_seconds,
    lockout_seconds=settings.login_rate_limit_lockout_seconds,
)
login_username_limiter = RateLimiter(
    max_attempts=settings.login_rate_limit_max_attempts,
    window_seconds=settings.login_rate_limit_window_seconds,
    lockout_seconds=settings.login_rate_limit_lockout_seconds,
)


def _enforce(limiter: RateLimiter, key: str) -> None:
    try:
        limiter.check(key)
    except RateLimitExceeded as exc:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail="too many attempts, please try again later",
            headers={"Retry-After": str(math.ceil(exc.retry_after))},
        ) from exc


@router.post("/register", response_model=UserOut, status_code=status.HTTP_201_CREATED)
def register(request: Request, payload: UserCreate, db: Session = Depends(get_db)) -> User:
    _enforce(register_ip_limiter, get_client_ip(request))

    user = User(
        username=payload.username,
        email=payload.email,
        hashed_password=hash_password(payload.password),
    )
    db.add(user)
    try:
        db.commit()
    except IntegrityError as exc:
        db.rollback()
        # Check if this is a duplicate username/email constraint violation.
        # SQLAlchemy wraps the DBAPI exception in exc.orig; both backends' error
        # text includes the field name ("users_username_key" on Postgres, "users.username"
        # on SQLite), so checking for the field name alone covers both.
        exc_str = str(exc.orig).lower()
        if "username" in exc_str or "email" in exc_str:
            detail = "username or email already registered"
        else:
            # Re-raise if it's some other integrity constraint we didn't expect
            raise
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=detail,
        ) from exc
    db.refresh(user)
    return user


@router.post("/login", response_model=Token)
def login(
    request: Request,
    form_data: OAuth2PasswordRequestForm = Depends(),
    db: Session = Depends(get_db),
) -> Token:
    client_ip = get_client_ip(request)
    username = form_data.username.lower()
    # Checked separately so a distributed brute force (many IPs, one username)
    # and a single-IP brute force (many usernames) are both caught.
    _enforce(login_ip_limiter, client_ip)
    _enforce(login_username_limiter, username)

    user = db.query(User).filter(User.username == username).first()
    password_hash = user.hashed_password if user is not None else DUMMY_PASSWORD_HASH
    # No real account's password can exceed 72 bytes (see
    # UserCreate._enforce_bcrypt_byte_limit), and an oversized value here is
    # never handed to bcrypt - swap it for a fixed-size placeholder so a
    # multi-megabyte payload can't reach passlib/bcrypt at all. verify_password
    # still runs unconditionally either way, so timing doesn't reveal whether
    # the length check tripped.
    password_bytes_ok = len(form_data.password.encode("utf-8")) <= 72
    password_to_check = form_data.password if password_bytes_ok else "oversized-password-rejected"
    password_matches = verify_password(password_to_check, password_hash)
    password_valid = password_bytes_ok and password_matches
    if user is None or not password_valid:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )

    # A successful login is strong evidence the caller is legitimate; clear
    # its attempt counters rather than let earlier failed attempts (typos,
    # etc.) count against it going forward.
    login_ip_limiter.reset(client_ip)
    login_username_limiter.reset(username)
    return Token(access_token=create_access_token(user.username))
