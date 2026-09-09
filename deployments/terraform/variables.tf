variable "aws_region" {
  type = string
  validation {
    condition     = trimspace(var.aws_region) != ""
    error_message = "aws_region must not be empty."
  }
}
variable "name" {
  type    = string
  default = "runbridge"
}
variable "image" {
  type        = string
  description = "Immutable container image URI"
  validation {
    condition     = trimspace(var.image) != ""
    error_message = "image must reference a container image."
  }
}
variable "cluster_arn" { type = string }
variable "subnet_ids" {
  type = list(string)
  validation {
    condition     = length(var.subnet_ids) > 0
    error_message = "at least one private subnet is required."
  }
}
variable "security_group_ids" {
  type = list(string)
  validation {
    condition     = length(var.security_group_ids) > 0
    error_message = "at least one security group is required."
  }
}
variable "execution_role_arn" {
  type = string
  validation {
    condition     = trimspace(var.execution_role_arn) != ""
    error_message = "execution_role_arn must not be empty."
  }
}
variable "task_role_arn" {
  type = string
  validation {
    condition     = trimspace(var.task_role_arn) != ""
    error_message = "task_role_arn must not be empty."
  }
}
variable "seqera_token_parameter_arn" {
  type      = string
  sensitive = true
}
variable "database_url_parameter_arn" {
  type      = string
  sensitive = true
}
variable "api_token_parameter_arn" {
  type        = string
  sensitive   = true
  description = "SSM SecureString ARN containing the RunBridge API token"
}
variable "webhook_secret_parameter_arn" {
  type        = string
  sensitive   = true
  description = "SSM SecureString ARN containing the Seqera webhook secret"
}
variable "actor_id" {
  type    = string
  default = "runbridge-service"
}
variable "actor_name" {
  type    = string
  default = "RunBridge service"
}
variable "desired_count" {
  type    = number
  default = 1
  validation {
    condition     = var.desired_count > 0
    error_message = "desired_count must be positive."
  }
}
variable "cpu" {
  type    = number
  default = 512
  validation {
    condition     = var.cpu > 0
    error_message = "cpu must be positive."
  }
}
variable "memory" {
  type    = number
  default = 1024
  validation {
    condition     = var.memory > 0
    error_message = "memory must be positive."
  }
}
