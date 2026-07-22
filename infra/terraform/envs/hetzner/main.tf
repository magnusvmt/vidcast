# Stage 2 of 2: installs the same platform module used by envs/local (Argo
# CD, CloudNativePG, namespaces) against the cluster provisioned by
# ../hetzner-cluster. Nothing here is Hetzner-specific - that's the point of
# reusing the module rather than forking it per environment.
module "platform" {
  source = "../../modules/platform"
}
