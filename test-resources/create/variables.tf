# =============================================================================
# Provider Configuration Variables
# =============================================================================

variable "truenas_host" {
  description = "TrueNAS host address (IP or hostname)"
  type        = string
}

variable "truenas_api_key" {
  description = "TrueNAS API key for authentication"
  type        = string
  sensitive   = true
}

variable "truenas_verify_ssl" {
  description = "Whether to verify SSL certificates"
  type        = bool
  default     = false
}

# =============================================================================
# Test Configuration Variables
# =============================================================================

variable "test_prefix" {
  description = "Prefix for all test resources to avoid naming conflicts"
  type        = string
  default     = "tftest"
}

variable "pool_name" {
  description = "Name of the pool to create for testing"
  type        = string
  default     = "testpool"
}

variable "pool_disks" {
  description = "List of disk identifiers to use for the test pool"
  type        = list(string)
}

variable "iscsi_listen_ip" {
  description = "IP address for iSCSI portal to listen on (use 0.0.0.0 for all)"
  type        = string
  default     = "0.0.0.0"
}

variable "nfs_allowed_network" {
  description = "Network CIDR allowed to access NFS shares (e.g., 192.168.1.0/24)"
  type        = string
}

variable "static_route_destination" {
  description = "Destination network for static route test (e.g., 10.0.0.0/8)"
  type        = string
  default     = "10.99.99.0/24"
}

variable "static_route_gateway" {
  description = "Gateway IP for static route test"
  type        = string
}

variable "test_user_password" {
  description = "Password for test user account"
  type        = string
  sensitive   = true
  default     = "TestPassword123!"
}

variable "test_app_name" {
  description = "Catalog app to deploy for testing (ix-app is the lightweight custom app)"
  type        = string
  default     = "ix-app"
}

variable "test_app_train" {
  description = "Catalog train for the test app"
  type        = string
  default     = "stable"
}

variable "test_app_version" {
  description = "Version of the test app to deploy"
  type        = string
  default     = "1.3.4"
}
