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

Platform: Kubernetes (k3d locally, k3s on Hetzner Cloud) · Terraform · Helm · GitHub Actions
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

## Cloud environment

`infra/terraform/envs/` holds one directory per target cluster. `local`
assumes a k3d cluster already exists (created by `make cluster`) and applies
the shared `platform` module against it. The cloud target follows the same
split, just with an extra step up front to create the cluster itself, since
that step doesn't exist for k3d:

1. `envs/hetzner-cluster` - provisions a single Hetzner Cloud server and
   bootstraps k3s on it via cloud-init, then writes its kubeconfig to
   `./kubeconfig` in that directory. Needs `TF_VAR_hcloud_token` (a Hetzner
   Cloud API token) and an SSH key pair (defaults to `~/.ssh/id_ed25519[.pub]`,
   override with `-var ssh_public_key_path=... -var ssh_private_key_path=...`).
2. `envs/hetzner` - applies the same `modules/platform` used by `local`
   against the kubeconfig `envs/hetzner-cluster` produced. Nothing in it is
   Hetzner-specific.

```
cd infra/terraform/envs/hetzner-cluster && terraform init && terraform apply
cd ../hetzner && terraform init && terraform apply
```

A single `cx22` server was chosen as the cheapest way to get a real k3s
cluster running; see `docs/adr/0002-hetzner-cloud-environment.md` for the
reasoning and what a managed-cluster alternative would have cost instead.
