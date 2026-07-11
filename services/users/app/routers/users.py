from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session, joinedload

from app.database import get_db
from app.deps import get_current_user
from app.models import Follow, User
from app.schemas import PublicUserOut, UserOut

router = APIRouter(prefix="/users", tags=["users"])


def _get_user_or_404(username: str, db: Session) -> User:
    user = db.query(User).filter(User.username == username).first()
    if user is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="user not found")
    return user


@router.get("/me", response_model=UserOut)
def read_current_user(current_user: User = Depends(get_current_user)) -> User:
    return current_user


@router.get("/{username}", response_model=PublicUserOut)
def read_user(username: str, db: Session = Depends(get_db)) -> User:
    return _get_user_or_404(username, db)


@router.post("/{username}/follow", status_code=status.HTTP_204_NO_CONTENT)
def follow_user(
    username: str,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
) -> None:
    target = _get_user_or_404(username, db)
    if target.id == current_user.id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="cannot follow yourself"
        )

    db.add(Follow(follower_id=current_user.id, followed_id=target.id))
    try:
        db.commit()
    except IntegrityError:
        # already following — a concurrent request won the race, treat as idempotent success
        db.rollback()


@router.delete("/{username}/follow", status_code=status.HTTP_204_NO_CONTENT)
def unfollow_user(
    username: str,
    current_user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
) -> None:
    target = _get_user_or_404(username, db)
    db.query(Follow).filter(
        Follow.follower_id == current_user.id, Follow.followed_id == target.id
    ).delete()
    db.commit()


@router.get("/{username}/followers", response_model=list[PublicUserOut])
def list_followers(
    username: str,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0),
    db: Session = Depends(get_db),
) -> list[User]:
    target = _get_user_or_404(username, db)
    follows = (
        db.query(Follow)
        .options(joinedload(Follow.follower))
        .filter(Follow.followed_id == target.id)
        .order_by(Follow.created_at)
        .offset(offset)
        .limit(limit)
        .all()
    )
    return [f.follower for f in follows]


@router.get("/{username}/following", response_model=list[PublicUserOut])
def list_following(
    username: str,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0),
    db: Session = Depends(get_db),
) -> list[User]:
    target = _get_user_or_404(username, db)
    follows = (
        db.query(Follow)
        .options(joinedload(Follow.followed))
        .filter(Follow.follower_id == target.id)
        .order_by(Follow.created_at)
        .offset(offset)
        .limit(limit)
        .all()
    )
    return [f.followed for f in follows]
