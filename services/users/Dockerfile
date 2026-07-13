ARG PYTHON_VERSION=3.12

FROM python:${PYTHON_VERSION}-slim AS build
WORKDIR /src
COPY requirements.txt ./
RUN pip install --no-cache-dir --target=/deps -r requirements.txt

FROM python:${PYTHON_VERSION}-slim
ARG PYTHON_VERSION
RUN useradd --uid 65532 --create-home nonroot
COPY --from=build /deps /usr/local/lib/python${PYTHON_VERSION}/site-packages
WORKDIR /app
COPY app ./app
USER nonroot:nonroot
EXPOSE 8080
CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
