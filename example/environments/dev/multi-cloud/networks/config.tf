data "terraform_remote_state" "aws-networks" {
  backend = "local"
  config = {
    path = "${path.module}/../../aws/global/terraform.tfstate"
  }
}

data "terraform_remote_state" "gcp-networks" {
  backend = "local"
  config = {
    path = "${path.module}/../../gcp/global/terraform.tfstate"
  }
}

output "mesh" {
  value = setproduct(
    data.terraform_remote_state.aws-networks.outputs.regions,
    data.terraform_remote_state.gcp-networks.outputs.regions,
  )
}