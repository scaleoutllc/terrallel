resource "target" "fast-dev" {
  group = [resource.target.network-dev-mesh]
  next {
    group = [resource.target.fast-dev-cluster-mesh]
  }
}

resource "target" "network-dev-mesh" {
  group = [
    resource.target.network-dev-aws-mesh,
    resource.target.network-dev-gcp,
  ]
  next {
    workspaces = ["shared/dev/multi-cloud/routing"]
  }
}

resource "target" "fast-dev-cluster-mesh" {
  group = [
    resource.target.fast-dev-gcp-cluster-mesh,
    resource.target.fast-dev-aws-cluster-mesh
  ]
}

