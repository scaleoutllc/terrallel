data "terraform_remote_state" "network" {
  backend = "local"
  config = {
    path = "${path.module}/../../network/terraform.tfstate"
  }
}

output "name" {
  value = "${data.terraform_remote_state.network.outputs.name}-cluster"
}