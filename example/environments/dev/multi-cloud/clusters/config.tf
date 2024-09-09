data "terraform_remote_state" "clusters" {
  for_each = toset([
    "aws/us-east-1",
    "aws/ap-southeast-2",
    "gcp/us-east1",
    "gcp/australia-southeast1",
  ])
  backend = "local"
  config = {
    path = "${path.module}/../../${each.key}/cluster/k8s/terraform.tfstate"
  }
}

output "mesh" {
  value = [
    for cluster in data.terraform_remote_state.clusters : cluster.outputs.name
  ]
}