resource "target" "fast-dev-gcp" {
  group = [resource.target.network-dev-gcp-mesh]
  next {
    group = [resource.target.fast-dev-gcp-cluster-mesh]
  }
}

resource "target" "network-dev-gcp-mesh" {
  workspaces = [
    "shared/dev/gcp/global/network",
    "fast/dev/gcp/global/network",
  ]
  next {
    group = values(resource.target.network-dev-gcp)
    next {
      workspaces = ["shared/dev/gcp/global/routing"]
    }
  }
}

resource "target" "network-dev-gcp" {
  for_each = toset(local.regions.gcp)
  next {
    workspaces = [
      "shared/dev/gcp/${each.key}/network",
      "fast/dev/gcp/${each.key}/network",
    ]
  }
}

resource "target" "fast-dev-gcp-cluster-mesh" {
  group = values(resource.target.fast-dev-gcp-cluster)
  next {
    workspaces = ["fast/dev/gcp/global/load-balancer"]
  }
}

resource "target" "fast-dev-gcp-cluster" {
  for_each   = toset(local.regions.gcp)
  workspaces = ["fast/dev/gcp/${each.key}/cluster"]
  next {
    workspaces = ["fast/dev/gcp/${each.key}/nodes"]
    next {
      workspaces = [
        "fast/dev/gcp/${each.key}/namespaces/autoneg-system",
        "fast/dev/gcp/${each.key}/namespaces/kube-system",
        "fast/dev/gcp/${each.key}/namespaces/istio-system",
      ]
      next {
        workspaces = ["fast/dev/gcp/${each.key}/namespaces/ingress"]
      }
    }
  }
}
