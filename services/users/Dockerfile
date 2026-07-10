FROM python:3.12-slim AS build
WORKDIR /src
COPY requirements.txt ./
RUN pip install --no-cache-dir --target=/deps -r requirements.txt

FROM python:3.12-slim
RUN useradd --uid 65532 --create-home nonroot
COPY --from=build /deps /usr/local/lib/python3.12/site-packages
WORKDIR /app
COPY app ./app
USER nonroot:nonroot
EXPOSE 8080
CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
