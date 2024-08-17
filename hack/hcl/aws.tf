resource "target" "fast-dev-aws" {
  group = [resource.target.network-dev-aws-mesh]
  next {
    group = [resource.target.fast-dev-aws-cluster-mesh]
  }
}

resource "target" "network-dev-aws-mesh" {
  group = values(resource.target.network-dev-aws)
  next {
    workspaces = ["shared/dev/aws/global/routing"]
  }
}

resource "target" "network-dev-aws" {
  for_each = toset(local.regions.aws)
  workspaces = [
    "shared/dev/aws/${each.value}/network",
    "fast/dev/aws/${each.value}/network",
  ]
}

resource "target" "fast-dev-aws-cluster-mesh" {
  workspaces = ["fast/dev/aws/global/accelerator"]
  next {
    group = values(resource.target.fast-dev-aws-cluster)
  }
}

resource "target" "fast-dev-aws-cluster" {
  for_each   = toset(local.regions.aws)
  workspaces = ["fast/dev/aws/${each.key}/cluster"]
  next {
    workspaces = ["fast/dev/aws/${each.key}/nodes"]
    next {
      workspaces = ["fast/dev/aws/${each.key}/namespaces/kube-system"]
      next {
        workspaces = ["fast/dev/aws/${each.key}/namespaces/istio-system"]
        next {
          workspaces = ["fast/dev/aws/${each.key}/namespaces/ingress"]
        }
      }
    }
  }
}
