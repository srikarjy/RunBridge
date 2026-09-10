variable "aws_region" {
  type = string
}

variable "name" {
  type    = string
  default = "runbridge"
}

variable "image" {
  type        = string
  description = "Immutable container image URI"
  validation {
    condition     = can(regex("@sha256:[0-9a-fA-F]{64}$", var.image))
    error_message = "image must use an immutable sha256 digest."
  }
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
  validation {
    condition     = length(var.private_subnet_ids) >= 2
    error_message = "at least two private subnets are required."
  }
}

variable "public_subnet_ids" {
  type = list(string)
  validation {
    condition     = length(var.public_subnet_ids) >= 2
    error_message = "at least two public subnets are required."
  }
}

variable "certificate_arn" {
  type = string
}

variable "ingress_cidr_blocks" {
  type    = list(string)
  default = ["0.0.0.0/0"]
}

variable "seqera_token_parameter_arn" {
  type      = string
  sensitive = true
}

variable "api_token_parameter_arn" {
  type      = string
  sensitive = true
}

variable "webhook_secret_parameter_arn" {
  type      = string
  sensitive = true
}

variable "secret_kms_key_arns" {
  type    = list(string)
  default = []
}

variable "seqera_base_url" {
  type    = string
  default = "https://api.cloud.seqera.io"
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
}

variable "memory" {
  type    = number
  default = 1024
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "database_name" {
  type    = string
  default = "runbridge"
}

variable "database_username" {
  type    = string
  default = "runbridge"
}

variable "postgres_engine_version" {
  type    = string
  default = "16"
}

variable "database_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "database_storage_gib" {
  type    = number
  default = 20
}

variable "database_max_storage_gib" {
  type    = number
  default = 100
}

variable "database_backup_retention_days" {
  type    = number
  default = 7
}

variable "database_deletion_protection" {
  type    = bool
  default = true
}
