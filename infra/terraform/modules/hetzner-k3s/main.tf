resource "hcloud_ssh_key" "default" {
  name       = "${var.cluster_name}-k3s"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

resource "hcloud_firewall" "k3s" {
  name = "${var.cluster_name}-k3s"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.allowed_ssh_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = var.allowed_k8s_api_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "k3s" {
  name         = "${var.cluster_name}-k3s"
  server_type  = var.server_type
  location     = var.location
  image        = var.image
  ssh_keys     = [hcloud_ssh_key.default.id]
  firewall_ids = [hcloud_firewall.k3s.id]

  # k3s auto-detects the node's own address (Hetzner Cloud servers get their
  # public IPv4 directly on eth0, no NAT) and adds it to the API server's TLS
  # SAN list, so no --tls-san / pre-reserved IP is needed here.
  user_data = templatefile("${path.module}/templates/cloud-init.yaml.tftpl", {
    k3s_channel = var.k3s_channel
  })
}

# k3s takes a minute or two to install after boot; retry the fetch until the
# kubeconfig file shows up rather than requiring a second `terraform apply`.
resource "null_resource" "kubeconfig" {
  triggers = {
    server_id              = hcloud_server.k3s.id
    ssh_private_key_path   = var.ssh_private_key_path
    kubeconfig_output_path = var.kubeconfig_output_path
  }

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    # StrictHostKeyChecking=no is a deliberate tradeoff for this hobby-scale
    # env: there's no side channel here to pre-fetch the host key for TOFU,
    # so the first connection to a freshly created server is unverified.
    command = <<-EOT
      set -euo pipefail
      ssh_key="${pathexpand(var.ssh_private_key_path)}"
      for i in $(seq 1 30); do
        if ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 \
            -i "$ssh_key" \
            root@${hcloud_server.k3s.ipv4_address} \
            'test -s /etc/rancher/k3s/k3s.yaml && grep -q apiVersion /etc/rancher/k3s/k3s.yaml'; then
          ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$ssh_key" \
            root@${hcloud_server.k3s.ipv4_address} \
            'cat /etc/rancher/k3s/k3s.yaml' \
            | sed "s/127.0.0.1/${hcloud_server.k3s.ipv4_address}/" \
            > "${var.kubeconfig_output_path}"
          chmod 600 "${var.kubeconfig_output_path}"
          exit 0
        fi
        sleep 10
      done
      echo "timed out waiting for k3s kubeconfig on ${hcloud_server.k3s.ipv4_address}" >&2
      exit 1
    EOT
  }
}
