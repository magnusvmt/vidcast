from fastapi import APIRouter, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordRequestForm
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.database import get_db
from app.models import User
from app.schemas import Token, UserCreate, UserOut
from app.security import DUMMY_PASSWORD_HASH, create_access_token, hash_password, verify_password

router = APIRouter(prefix="/auth", tags=["auth"])


@router.post("/register", response_model=UserOut, status_code=status.HTTP_201_CREATED)
def register(payload: UserCreate, db: Session = Depends(get_db)) -> User:
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
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="username or email already registered",
        ) from exc
    db.refresh(user)
    return user


@router.post("/login", response_model=Token)
def login(
    form_data: OAuth2PasswordRequestForm = Depends(), db: Session = Depends(get_db)
) -> Token:
    user = db.query(User).filter(User.username == form_data.username.lower()).first()
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

    return Token(access_token=create_access_token(user.username))
