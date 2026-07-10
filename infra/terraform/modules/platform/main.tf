resource "kubernetes_namespace" "apps" {
  metadata {
    name = "apps"
  }
}

resource "kubernetes_namespace" "platform" {
  metadata {
    name = "platform"
  }
}
