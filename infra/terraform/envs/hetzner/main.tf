# Stage 2 of 2: installs the same platform module used by envs/local (Argo
# CD, CloudNativePG, namespaces) against the cluster provisioned by
# ../hetzner-cluster. Nothing here is Hetzner-specific - that's the point of
# reusing the module rather than reimplementing it per environment.
#
# minio_root_password is intentionally omitted here — it is a required
# variable with no default (see ../../modules/platform/variables.tf).
# Provide it via -var or a .tfvars file (gitignored) at apply time, e.g.:
#   terraform apply -var=minio_root_password="<your-secret>"
# or create infra/terraform/envs/hetzner/terraform.tfvars with:
#   minio_root_password = "<your-secret>"
#
# This prevents a well-known dev password from silently reaching the
# long-lived Hetzner environment. envs/local/main.tf hardcodes a dev
# password because it is only used for local development.
module "platform" {
  source = "../../modules/platform"
}