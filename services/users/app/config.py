from urllib.parse import quote_plus

from pydantic import model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

_INSECURE_DEFAULT_JWT_SECRET = "dev-secret-do-not-use-in-production"


class Settings(BaseSettings):
    model_config = SettingsConfigDict()

    database_url: str = "sqlite:////tmp/users-dev.db"
    db_host: str | None = None
    db_port: int = 5432
    db_name: str | None = None
    db_user: str | None = None
    db_password: str | None = None

    environment: str = "development"
    jwt_secret: str = _INSECURE_DEFAULT_JWT_SECRET
    jwt_algorithm: str = "HS256"
    jwt_expire_minutes: int = 60
    version: str = "dev"

    @model_validator(mode="after")
    def _reject_insecure_secret_outside_dev(self) -> "Settings":
        if self.environment != "development" and self.jwt_secret == _INSECURE_DEFAULT_JWT_SECRET:
            raise ValueError(
                "JWT_SECRET must be set to a real secret when ENVIRONMENT is not 'development'"
            )
        return self

    @model_validator(mode="after")
    def _require_discrete_db_fields_together(self) -> "Settings":
        discrete_fields = (self.db_user, self.db_password, self.db_name)
        if self.db_host and not all(discrete_fields):
            raise ValueError(
                "DB_USER, DB_PASSWORD, and DB_NAME must all be set when DB_HOST is set"
            )
        return self

    @property
    def resolved_database_url(self) -> str:
        # CloudNativePG's generated app secret exposes discrete host/port/dbname/user/password
        # keys rather than a single URI, and its URI uses the psycopg2 scheme, not psycopg3's.
        if self.db_host:
            user = quote_plus(self.db_user)
            password = quote_plus(self.db_password)
            return f"postgresql+psycopg://{user}:{password}@{self.db_host}:{self.db_port}/{self.db_name}"
        return self.database_url


settings = Settings()
