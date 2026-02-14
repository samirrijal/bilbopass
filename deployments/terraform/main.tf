# BilboPass Terraform — GKE + Cloud SQL
# This is a starter scaffold. Adapt to your cloud provider.

terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  backend "gcs" {
    bucket = "bilbopass-terraform-state"
    prefix = "prod"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "db_password" {
  type      = string
  sensitive = true
}

# --- GKE Cluster ---

resource "google_container_cluster" "primary" {
  name     = "bilbopass-cluster"
  location = var.region

  initial_node_count = 3

  node_config {
    machine_type = "e2-standard-4"
    disk_size_gb = 50

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  release_channel {
    channel = "REGULAR"
  }
}

# --- Cloud SQL (TimescaleDB-compatible PostgreSQL) ---

resource "google_sql_database_instance" "timescale" {
  name             = "bilbopass-db"
  database_version = "POSTGRES_16"
  region           = var.region

  settings {
    tier              = "db-custom-4-16384"
    availability_type = "REGIONAL"

    database_flags {
      name  = "shared_preload_libraries"
      value = "timescaledb"
    }

    backup_configuration {
      enabled    = true
      start_time = "03:00"
    }

    ip_configuration {
      ipv4_enabled = false
      private_network = google_compute_network.vpc.id
    }
  }
}

resource "google_sql_database" "bilbopass" {
  name     = "bilbopass"
  instance = google_sql_database_instance.timescale.name
}

resource "google_sql_user" "transit" {
  name     = "transit"
  instance = google_sql_database_instance.timescale.name
  password = var.db_password
}

# --- VPC ---

resource "google_compute_network" "vpc" {
  name                    = "bilbopass-vpc"
  auto_create_subnetworks = true
}

# --- Outputs ---

output "cluster_endpoint" {
  value = google_container_cluster.primary.endpoint
}

output "db_connection_name" {
  value = google_sql_database_instance.timescale.connection_name
}
