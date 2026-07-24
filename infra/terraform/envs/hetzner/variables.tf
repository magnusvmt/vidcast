variable "kubeconfig_path" {
  description = "Path to the kubeconfig for the target k3s cluster, produced by envs/hetzner-cluster (see its ./kubeconfig output)."
  type        = string
  default     = "../hetzner-cluster/kubeconfig"
}
