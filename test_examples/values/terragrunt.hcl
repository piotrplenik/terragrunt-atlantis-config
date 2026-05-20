locals {
  source_base_url   = "git::git@github.com:transcend-io/terraform-aws-fargate-container"
  default_role_name = "${values.environment}-${values.project}-eks-${basename(get_terragrunt_dir())}"
}

terraform {
  source = "${local.source_base_url}?ref=${values.module_ref}"
}

inputs = {
  foo = "${values.foo}-${values.bar}"
}