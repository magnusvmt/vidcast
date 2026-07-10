from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict()

    database_url: str = "sqlite:////tmp/users-dev.db"
    db_host: str | None = None
    db_port: int = 5432
    db_name: str | None = None
    db_user: str | None = None
    db_password: str | None = None

    jwt_secret: str = "dev-secret-do-not-use-in-production"
    jwt_algorithm: str = "HS256"
    jwt_expire_minutes: int = 60
    version: str = "dev"

    @property
    def resolved_database_url(self) -> str:
        # CloudNativePG's generated app secret exposes discrete host/port/dbname/user/password
        # keys rather than a single URI, and its URI uses the psycopg2 scheme, not psycopg3's.
        if self.db_host:
            return (
                f"postgresql+psycopg://{self.db_user}:{self.db_password}"
                f"@{self.db_host}:{self.db_port}/{self.db_name}"
            )
        return self.database_url


settings = Settings()
