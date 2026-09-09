variable "aws_region" { type = string }
variable "name" {
  type    = string
  default = "runbridge"
}
variable "image" {
  type        = string
  description = "Immutable container image URI"
}
variable "cluster_arn" { type = string }
variable "subnet_ids" { type = list(string) }
variable "security_group_ids" { type = list(string) }
variable "execution_role_arn" { type = string }
variable "task_role_arn" { type = string }
variable "seqera_token_parameter_arn" {
  type      = string
  sensitive = true
}
variable "database_url_parameter_arn" {
  type      = string
  sensitive = true
}
variable "cpu" {
  type    = number
  default = 512
}
variable "memory" {
  type    = number
  default = 1024
}
