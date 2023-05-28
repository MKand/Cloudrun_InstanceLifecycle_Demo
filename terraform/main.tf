terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.51.0"
    }
  }
   backend "gcs" {
   bucket  = "cloudrun-lifecycle-demo"
   prefix  = "terraform/state"
 }
}

provider "google" {
  project     = var.project_id
  region      = var.region
  zone        = var.zone
}


resource "google_service_account" "sa-name-publisher" {
  account_id = "hello-topic-publisher"
}

resource "google_service_account" "sa-name-subscriber" {
  account_id = "hello-topic-subscriber"
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

resource "google_cloud_run_service" "api" {
  name     = "hello-service"
  location = var.region

  template {
    spec {
      container_concurrency = 10
      service_account_name = google_service_account.sa-name-publisher.email
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repo}/helloapi:latest"
        ports {
          container_port = "8080"
        }
        env {
          name  = "TOPIC_NAME"
          value = var.topic_name
        }
        env {
          name  = "PROJECT_ID"
          value = var.project_id
        }
        env{
          name = "MESSAGE_INTERVAL"
          value = var.message_publish_interval
        }
        env{
          name = "RESPONSE_DELAY_INTERVAL"
          value = var.response_delay_interval
        }
    }
    }
    }
  traffic {
    percent         = 100
    latest_revision = true
  }
  
depends_on = [
  google_pubsub_topic.hello-topic, google_project_iam_member.pubsub_publisher_binding
]
}

resource "google_cloud_run_service" "visualizer" {
  name     = "hello-visualizer"
  location = var.region

  template {
    spec {
      container_concurrency = 80
      service_account_name = google_service_account.sa-name-subscriber.email
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repo}/visualizer:latest"
        ports {
          container_port = "8080"
        }
        env {
          name  = "TOPIC_NAME"
          value = var.topic_name
        }
        env {
          name  = "PROJECT_ID"
          value = var.project_id
        }
      }
    }
  }
  traffic {
    percent         = 100
    latest_revision = true
  }
  
depends_on = [
  google_pubsub_topic.hello-topic, google_project_iam_member.pubsub_editor_binding
]
}


data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_service_iam_policy" "noauth_api" {
  location    = google_cloud_run_service.api.location
  project     = google_cloud_run_service.api.project
  service     = google_cloud_run_service.api.name
  policy_data = data.google_iam_policy.noauth.policy_data
}

resource "google_cloud_run_service_iam_policy" "noauth_visualizer" {
  location    = google_cloud_run_service.visualizer.location
  project     = google_cloud_run_service.visualizer.project
  service     = google_cloud_run_service.visualizer.name
  policy_data = data.google_iam_policy.noauth.policy_data
}


resource "google_pubsub_topic" "hello-topic" {
  name                       = var.topic_name
  message_retention_duration = "86600s"
}

resource "google_pubsub_subscription" "subscription" {
  name  = var.subscription_name
  topic = google_pubsub_topic.hello-topic.name
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