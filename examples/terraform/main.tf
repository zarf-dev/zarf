terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
      version = "4.33.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

resource "aws_s3_bucket" "example-bucket" {
  # note this bucket name will need to be changed because s3 buckets must be globally unique
  bucket = "unclegedds-example-bucket"
}
