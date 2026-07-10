# ADR-0001: Platform Foundations

## Status
Accepted

## Context
Vidcast needs a foundation for how it is built, deployed, and operated before
any application feature work begins. The following decisions set that
foundation.

## Decisions

**Kubernetes for orchestration.** The platform runs on Kubernetes throughout
its life, starting locally on k3d (k3s in Docker) and moving to a cloud
provider once the local setup is proven out.

**Terraform for infrastructure.** Terraform provisions and configures
everything above the raw cluster (namespaces, platform services, and later
cloud resources). Ansible is not used — there are no bare VMs to configure
here, and Terraform's Kubernetes/Helm providers cover the same ground more
directly.

**Polyglot services.** Each microservice is written in the language best
suited to it rather than standardizing on one. This keeps shared platform
concerns (CI, Helm charts, observability) decoupled from any single
language's tooling.

**Off-the-shelf media server.** Live video ingest/playback uses MediaMTX
rather than a custom-built RTMP/HLS pipeline. Writing a media server is a
large, separate problem from building the platform around it.

**Test-driven development.** Application code is written test-first: a
failing test before the implementation that satisfies it. This applies to
every service, in every phase.

**CI from day one.** GitHub Actions runs lint, test, and build on every
change starting with the very first service, using free GitHub-hosted
runners (the repo is public).

## Consequences
- The repo structure separates `infra/` (Terraform + cluster config),
  `deploy/` (Helm charts), and `services/` (application code) so each can
  evolve independently.
- A shared Helm library chart (`deploy/charts/vidcast-lib`) is used by every
  service to avoid duplicating Deployment/Service/Ingress boilerplate.
- Moving to a cloud provider later should mainly mean adding a new Terraform
  env and reusing the same modules and charts.
