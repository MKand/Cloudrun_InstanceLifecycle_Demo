variable "project_id" {
  type = string
}

variable "region" {
  type    = string
}

variable "zone" {
  type    = string
}

variable "topic_name" {
  type    = string
  default = "hello-topic"
}

variable "subscription_name" {
  type    = string
  default = "hello-subscription"
}