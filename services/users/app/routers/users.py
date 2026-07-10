from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.database import get_db
from app.deps import get_current_user
from app.models import Follow, User
from app.schemas import UserOut

router = APIRouter(prefix="/users", tags=["users"])


def _get_user_or_404(username: str, db: Session) -> User:
    user = db.query(User).filter(User.username == username).first()
    if user is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="user not found")
    return user


@router.get("/me", response_model=UserOut)
def read_current_user(current_user: User = Depends(get_current_user)) -> User:
    return current_user


@router.get("/{username}", response_model=UserOut)
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

    exists = (
        db.query(Follow)
        .filter(Follow.follower_id == current_user.id, Follow.followed_id == target.id)
        .first()
    )
    if exists is None:
        db.add(Follow(follower_id=current_user.id, followed_id=target.id))
        db.commit()


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


@router.get("/{username}/followers", response_model=list[UserOut])
def list_followers(username: str, db: Session = Depends(get_db)) -> list[User]:
    target = _get_user_or_404(username, db)
    return [f.follower for f in db.query(Follow).filter(Follow.followed_id == target.id).all()]


@router.get("/{username}/following", response_model=list[UserOut])
def list_following(username: str, db: Session = Depends(get_db)) -> list[User]:
    target = _get_user_or_404(username, db)
    return [f.followed for f in db.query(Follow).filter(Follow.follower_id == target.id).all()]
