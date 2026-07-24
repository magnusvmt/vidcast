variable "kubeconfig_path" {
  description = "Path to the kubeconfig for the target k3s cluster, produced by envs/hetzner-cluster (see its ./kubeconfig output)."
  type        = string
  default     = "../hetzner-cluster/kubeconfig"
}

variable "minio_root_password" {
  description = "Root password for the in-cluster MinIO instance (VOD object storage). No default — must be provided explicitly, e.g. via -var or a gitignored terraform.tfvars file."
  type        = string
  sensitive   = true
}
