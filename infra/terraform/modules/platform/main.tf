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

resource "helm_release" "minio" {
  # Deployed to "apps" rather than "platform": unlike CloudNativePG (an
  # operator with no data-plane footprint of its own), MinIO here *is* the
  # data-plane object store that apps-namespace workloads (mediamtx,
  # streams) read/write directly - keeping it in the same namespace lets
  # their pods reference its generated credentials Secret via a plain
  # secretKeyRef instead of needing a cross-namespace copy.
  name       = "minio"
  namespace  = kubernetes_namespace.apps.metadata[0].name
  repository = "https://charts.min.io/"
  chart      = "minio"
  version    = "5.4.0"

  wait    = true
  timeout = 180

  set {
    name  = "mode"
    value = "standalone"
  }
  set_sensitive {
    name  = "rootUser"
    value = var.minio_root_user
  }
  set_sensitive {
    name  = "rootPassword"
    value = var.minio_root_password
  }
  # A single small PVC is plenty for local/dev VOD recordings; distributed
  # mode's chart defaults (16Gi mem requests, 500Gi PVCs) are wildly
  # oversized for standalone mode.
  set {
    name  = "persistence.size"
    value = "20Gi"
  }
  set {
    name  = "resources.requests.memory"
    value = "512Mi"
  }
  set {
    name  = "buckets[0].name"
    value = "vod-recordings"
  }
  set {
    name  = "buckets[0].policy"
    value = "none"
  }
  set {
    name  = "buckets[0].purge"
    value = "false"
  }
}

resource "helm_release" "argocd" {
  name       = "argocd"
  namespace  = kubernetes_namespace.platform.metadata[0].name
  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argo-cd"
  version    = "10.1.4"

  wait    = true
  timeout = 300

  set {
    name  = "dex.enabled"
    value = "false"
  }
  set {
    name  = "notifications.controller.enabled"
    value = "false"
  }
  set {
    name  = "applicationset.enabled"
    value = "false"
  }
}
