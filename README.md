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
deploy/argocd/       ArgoCD app-of-apps: root Application + one per service
services/            application services, one directory each
.github/workflows/   CI
```

## Status

Early scaffolding stage. See `docs/adr/` for foundational decisions.

## Quickstart

```
make hooks             # enable repo git hooks (run once per clone)
make cluster           # create the local k3d cluster
make infra             # terraform apply against it (installs ArgoCD, among other platform services)
make argocd-bootstrap  # point ArgoCD at deploy/argocd/apps (one-time)
make deploy            # build and push the echo service's dev image (only before bootstrapping)
make deploy-chat       # build and push the chat service's dev image (only before bootstrapping)
make deploy-users      # build and push the users service's dev image (only before bootstrapping)
make destroy           # tear everything down
```

> **Note:** After `make argocd-bootstrap` runs, every Application with
> `selfHeal: true` (echo, chat, users, mediamtx) is continuously reconciled from
> its chart in git. Any local `helm upgrade --install` (including
> `make deploy`, `make deploy-chat`, `make deploy-users`) will be silently
> reverted on the next sync cycle. Use these targets only *before* bootstrapping,
> or push chart changes to git instead.

Once bootstrapped, ArgoCD reconciles every Helm release under `deploy/charts/`
that has a matching Application manifest in `deploy/argocd/apps/` - adding a
new service there is enough to bring it under GitOps management, no extra
`helm upgrade --install` required.
