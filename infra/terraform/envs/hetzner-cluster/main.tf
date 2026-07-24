# Stage 1 of 2 for the Hetzner cloud environment: provisions the VM and
# bootstraps k3s on it via cloud-init, writing a kubeconfig to ./kubeconfig.
# Run this to completion, then apply ../hetzner (stage 2) to install the
# shared platform module against the resulting cluster - mirroring how
# envs/local assumes a k3d cluster/kubeconfig already exist before its own
# apply runs.
module "cluster" {
  source = "../../modules/hetzner-k3s"

  server_type           = var.server_type
  location              = var.location
  ssh_public_key_path   = var.ssh_public_key_path
  ssh_private_key_path  = var.ssh_private_key_path
  allowed_ssh_cidrs     = var.allowed_ssh_cidrs
  allowed_k8s_api_cidrs = var.allowed_k8s_api_cidrs
}
