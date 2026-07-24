output "server_ipv4" {
  description = "Public IPv4 address of the k3s node."
  value       = hcloud_server.k3s.ipv4_address
}

output "kubeconfig_path" {
  description = "Local path the kubeconfig was written to. Only valid after the null_resource.kubeconfig provisioner has run."
  value       = var.kubeconfig_output_path
  depends_on  = [null_resource.kubeconfig]
}
