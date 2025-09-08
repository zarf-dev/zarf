provider "aws" {
  region = "{{ .Values.terraform.aws.region }}"
}

terraform {
  required_version = ">= 0.13"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.30.0"
    }
  }
}
