data "terraform_remote_state" "cluster" {
  backend = "local"
  config = {
    path = "${path.module}/../k8s/terraform.tfstate"
  }
}

output "services" {
  value = [
    "${data.terraform_remote_state.cluster.outputs.name}-cert-manager",
    "${data.terraform_remote_state.cluster.outputs.name}-istio",
  ]
}