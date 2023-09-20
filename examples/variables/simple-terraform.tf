provider "aws" {
  region = "###ZARF_VAR_AWS_REGION###"
}

terraform {
  required_version = ">= 0.13"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.17.0"
    }
  }
}
