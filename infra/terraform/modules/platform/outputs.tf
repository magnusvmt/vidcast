output "apps_namespace" {
  value = kubernetes_namespace.apps.metadata[0].name
}

output "platform_namespace" {
  value = kubernetes_namespace.platform.metadata[0].name
}
