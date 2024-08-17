// just an exploration of what this config would look like in terraform.
locals {
  providers = ["aws", "gcp"]
  regions = {
    aws = ["us-east-1", "ap-southeast-2"]
    gcp = ["us-east1", "australia-southeast1"]
  }
}
