locals {
  regions = ["us-east1", "australia-southeast1"]
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