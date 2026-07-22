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

## GitOps image bump

CI never runs `helm upgrade` against the cluster. For echo, chat, and users,
each Application manifest (`deploy/argocd/apps/{service}.yaml`) overrides the
chart's own `image.repository`/`image.tag` to point at that service's image on
GHCR, instead of the local dev registry the chart defaults to. On every push
to `main` that changes a service, CI publishes its image to GHCR and then a
`bump-image-tags` job edits that service's Application manifest to the new
commit SHA, opens a PR, and (if enabled - see below) auto-merges it. ArgoCD's
existing automated sync + selfHeal then reconciles the cluster from the merged
manifest - CI never touches the cluster directly.

Two repo settings gate the unattended part of this loop; without them the
PR may still open (if creation is allowed), it just needs a human to merge it:
- Settings → Actions → General → Workflow permissions → "Allow GitHub Actions
  to create and approve pull requests" (without this, `gh pr create` fails
  and the job exits with a non-fatal message)
- Settings → General → Pull Requests → "Allow auto-merge"

The chart's own `values.yaml` (used by `make deploy`, `make deploy-chat`,
`make deploy-users`) is untouched by this and still points at the local k3d
registry - the pre-bootstrap local flow above is unaffected.
