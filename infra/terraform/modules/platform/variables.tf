# Dev-friendly defaults so envs/local and envs/hetzner can both keep
# invoking this module with zero arguments (see the comment in
# envs/hetzner/main.tf on why nothing here is Hetzner-specific). Override
# via -var/tfvars for any environment that needs different credentials.
variable "minio_root_user" {
  description = "Root username for the in-cluster MinIO instance (VOD object storage)."
  type        = string
  default     = "vidcast"
}

variable "minio_root_password" {
  description = "Root password for the in-cluster MinIO instance (VOD object storage)."
  type        = string
  default     = "vidcast-minio-dev"
  sensitive   = true
}
