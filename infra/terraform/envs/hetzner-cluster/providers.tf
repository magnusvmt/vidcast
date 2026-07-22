terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.49"
    }
  }
}

variable "hcloud_token" {
  description = "Hetzner Cloud API token. Set via TF_VAR_hcloud_token, not a .tfvars file."
  type        = string
  sensitive   = true
}

provider "hcloud" {
  token = var.hcloud_token
}
