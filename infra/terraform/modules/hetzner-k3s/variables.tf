variable "cluster_name" {
  description = "Prefix for all Hetzner resources created by this module."
  type        = string
  default     = "vidcast"
}

variable "server_type" {
  description = "Hetzner Cloud server type for the k3s node. cx22 is the cheapest shared-vCPU type as of writing."
  type        = string
  default     = "cx22"
}

variable "location" {
  description = "Hetzner Cloud location for the server."
  type        = string
  default     = "nbg1"
}

variable "image" {
  description = "OS image for the server."
  type        = string
  default     = "ubuntu-24.04"
}

variable "k3s_channel" {
  description = "k3s release channel passed to the install script (see https://get.k3s.io)."
  type        = string
  default     = "stable"
}

variable "ssh_public_key_path" {
  description = "Path to the local SSH public key uploaded to Hetzner and granted access to the server."
  type        = string
}

variable "ssh_private_key_path" {
  description = "Path to the local SSH private key matching ssh_public_key_path, used once to pull the k3s kubeconfig off the server."
  type        = string
}

variable "allowed_ssh_cidrs" {
  description = "CIDR blocks allowed to reach the server on port 22. No default — must be explicitly set to a specific IP range."
  type        = list(string)
}

variable "allowed_k8s_api_cidrs" {
  description = "CIDR blocks allowed to reach the k3s API server on port 6443. No default — must be explicitly set to a specific IP range."
  type        = list(string)
}

variable "kubeconfig_output_path" {
  description = "Local path to write the fetched kubeconfig to."
  type        = string
  default     = "kubeconfig"
}
