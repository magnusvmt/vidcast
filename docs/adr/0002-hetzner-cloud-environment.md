# ADR-0002: Hetzner Cloud for the first cloud environment

## Status
Accepted

## Context
ADR-0001 anticipated moving off k3d to a real cloud provider once the local
setup was proven out, and said that step "should mainly mean adding a new
Terraform env and reusing the same modules and charts." It's now time to add
that env, and the concrete provider has to be picked cost-driven, since this
project pays its own cloud bill.

The two shapes considered:
- **A managed Kubernetes control plane** (EKS, GKE, DigitalOcean Kubernetes,
  etc.). Removes control-plane operational work, but either charges for the
  control plane directly or requires worker nodes priced well above
  Hetzner's, and pulls in a second cloud account/IAM model for a single-node
  hobby-scale cluster that doesn't need HA control planes yet.
- **Self-managed k3s on plain VMs.** k3s is already the local dev target
  (k3d runs k3s in Docker), so there's no new distribution to learn. Hetzner
  Cloud's smallest shared-vCPU server (`cx22`) runs it for a few euros a
  month, with no separate control-plane charge.

## Decision
Provision a single Hetzner Cloud server via Terraform (`hcloud` provider) and
install k3s on it with a cloud-init script, rather than paying for a managed
control plane. This is split into two Terraform environments:

- `infra/terraform/envs/hetzner-cluster` provisions the server/firewall/SSH
  key and bootstraps k3s, writing a kubeconfig locally.
- `infra/terraform/envs/hetzner` applies the existing `modules/platform`
  (namespaces, CloudNativePG, Argo CD) against that kubeconfig - identical to
  what `envs/local` does against k3d.

Splitting them avoids a chicken-and-egg problem: the `kubernetes`/`helm`
Terraform providers need a kubeconfig at provider-configuration time, which
can't come from a resource created later in the same apply.

## Consequences
- Reuses `modules/platform` and every Helm chart under `deploy/charts/`
  unchanged - the point of ADR-0001's prediction.
- A single node is not highly available; that's an acceptable tradeoff at
  current scale and can be revisited (e.g. adding k3s agents, or migrating to
  a managed control plane) without touching `modules/platform` or the charts.
- CI only runs `terraform validate` against these envs (as it already did for
  `local`) - it has no Hetzner credentials and does not provision real
  infrastructure. Applying for real is a manual, cost-incurring action.
