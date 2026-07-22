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

variable "ssh_public_key_path" {
  description = "Path to the local SSH public key uploaded to Hetzner and granted access to the server."
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "ssh_private_key_path" {
  description = "Path to the local SSH private key matching ssh_public_key_path, used once to pull the k3s kubeconfig off the server."
  type        = string
  default     = "~/.ssh/id_ed25519"
}

variable "allowed_ssh_cidrs" {
  description = "CIDR blocks allowed to reach the server on port 22. No default — must be explicitly set to a specific IP range (e.g. [\"Your.IP.Address/32\"])."
  type        = list(string)
}

variable "allowed_k8s_api_cidrs" {
  description = "CIDR blocks allowed to reach the k3s API server on port 6443."
  type        = list(string)
  default     = ["0.0.0.0/0", "::/0"]
}
