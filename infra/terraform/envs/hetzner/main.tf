# Stage 2 of 2: installs the same platform module used by envs/local (Argo
# CD, CloudNativePG, namespaces) against the cluster provisioned by
# ../hetzner-cluster. Nothing here is Hetzner-specific - that's the point of
# reusing the module rather than forking it per environment.
#
# WARNING: the dev password below is not suitable for production. Override
# minio_root_password via -var or a .tfvars file when deploying to a real
# environment.
module "platform" {
  source = "../../modules/platform"

  minio_root_password = "vidcast-minio-dev"
}