terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.51.0"
    }
  }
   backend "gcs" {
   bucket  = "cloudrun-lifecycle-demo-local"
   prefix  = "terraform/state"
 }
}

provider "google" {
  project     = var.project_id
  region      = var.region
  zone        = var.zone
}

resource "google_service_account" "sa-name-publisher" {
  account_id = "hello-topic-publisher-local"
}

resource "google_service_account" "sa-name-subscriber" {
  account_id = "hello-topic-subscriber-local"
}

resource "google_project_iam_member" "pubsub_publisher_binding" {
  project = var.project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.sa-name-publisher.email}"
}

resource "google_project_iam_member" "pubsub_editor_binding" {
  project = var.project_id
  role    = "roles/pubsub.editor"
  member  = "serviceAccount:${google_service_account.sa-name-subscriber.email}"
}

resource "google_pubsub_topic" "hello-topic-localhost" {
  name                       = "${var.topic_name}-local"
  message_retention_duration = "86600s"
}

resource "google_pubsub_subscription" "subscription" {
  name  = "${var.subscription_name}-local"
  topic = google_pubsub_topic.hello-topic-localhost.name
  message_retention_duration = "1200s"
  retain_acked_messages      = true

  ack_deadline_seconds = 20

  expiration_policy {
    ttl = "300000.5s"
  }
  retry_policy {
    minimum_backoff = "10s"
  }
  enable_message_ordering    = false
}