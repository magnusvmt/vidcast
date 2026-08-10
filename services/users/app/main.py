import asyncio
from contextlib import asynccontextmanager

from fastapi import FastAPI
from sqlalchemy import text

from app.config import settings
from app.database import engine
from app.migrations import upgrade_to_head
from app.routers import auth, users


@asynccontextmanager
async def lifespan(app: FastAPI):
    loop = asyncio.get_running_loop()
    await loop.run_in_executor(None, upgrade_to_head, engine)
    for limiter in (auth.register_ip_limiter, auth.login_ip_limiter, auth.login_username_limiter):
        limiter.reset_all()
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
