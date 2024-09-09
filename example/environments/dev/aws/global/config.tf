locals {
  regions = ["us-east-1", "ap-southeast-2"]
}

data "terraform_remote_state" "network" {
  for_each = toset(local.regions)
  backend  = "local"
  config = {
    path = "${path.module}/../${each.value}/network/terraform.tfstate"
  }
}

output "regions" {
  value = local.regions
}

output "peering" {
  value = join("-to-", [for network in data.terraform_remote_state.network : network.outputs.name])
}