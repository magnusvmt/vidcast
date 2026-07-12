from contextlib import asynccontextmanager

from fastapi import FastAPI
from sqlalchemy import text

from app.config import settings
from app.database import Base, engine
from app.routers import auth, users


@asynccontextmanager
async def lifespan(app: FastAPI):
    Base.metadata.create_all(bind=engine)
    yield


app = FastAPI(
    title="users",
    lifespan=lifespan,
    docs_url="/docs" if settings.environment == "development" else None,
    redoc_url="/redoc" if settings.environment == "development" else None,
    openapi_url="/openapi.json" if settings.environment == "development" else None,
)
app.include_router(auth.router)
app.include_router(users.router)


@app.get("/")
def root() -> dict:
    return {"service": "users", "version": settings.version}


@app.get("/healthz")
def healthz() -> dict:
    with engine.connect() as conn:
        conn.execute(text("SELECT 1"))
    return {"status": "ok"}
