# Vidcast

Vidcast is a live-streaming platform: viewers watch live broadcasts and chat
with each other in real time.

## Architecture

```
OBS ──RTMP──> MediaMTX ──HLS──> viewers (web frontend, HLS.js)
                 │ auth webhook
                 ▼
        ┌─ streams svc ──── stream keys, live-channel registry
        ├─ users svc ────── accounts, auth, follows ──── Postgres
        ├─ chat svc ─────── WebSockets, pub/sub fan-out ─ Redis
        └─ web ──────────── channel pages, player + chat

Platform: Kubernetes (k3d locally) · Terraform · Helm · GitHub Actions
```

## Repo layout

```
docs/adr/            architecture decision records
infra/k3d/           local cluster definition
infra/terraform/     Terraform modules and environments
deploy/charts/       Helm charts (shared library chart + one per service)
services/            application services, one directory each
.github/workflows/   CI
```

## Status

Early scaffolding stage. See `docs/adr/` for foundational decisions.

## Quickstart

```
make hooks     # enable repo git hooks (run once per clone)
make cluster   # create the local k3d cluster
make infra     # terraform apply against it
make deploy    # build and deploy services
make destroy   # tear everything down
```
