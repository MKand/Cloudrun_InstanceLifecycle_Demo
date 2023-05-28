variable "project_id" {
  type = string
  description = "Your GCP project id."
}

variable "region" {
  type    = string
}

variable "zone" {
  type    = string
}

variable "artifact_registry_repo" {
  type = string
  description = "The name of the repo that will store your container images."
}

variable "topic_name" {
  type    = string
  description = "Name of the topic to be created for the HelloApi service."
}

variable "subscription_name" {
  type    = string
  description = "Name of the subscription to be created for the HelloApi service."
}

variable "message_publish_interval" {
  type = number
  description = "How often should HelloAPI publish messages, in seconds." 
}

variable "response_delay_interval" {
  type = number
  description = "An artificial delay in seconds that the HelloAPI creates while responding to a user request. A longer delay helps users see the Active Instances displayed on screen for longer." 
}