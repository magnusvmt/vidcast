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

resource "helm_release" "cloudnative_pg" {
  name       = "cloudnative-pg"
  namespace  = kubernetes_namespace.platform.metadata[0].name
  repository = "https://cloudnative-pg.github.io/charts"
  chart      = "cloudnative-pg"
  version    = "0.22.1"

  wait    = true
  timeout = 180
}

resource "helm_release" "argocd" {
  name       = "argocd"
  namespace  = kubernetes_namespace.platform.metadata[0].name
  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argo-cd"
  version    = "10.1.4"

  wait    = true
  timeout = 300
}
