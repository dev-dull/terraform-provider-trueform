# TrueNAS Connection
truenas_host       = "192.168.1.100"
truenas_api_key    = "1-0123456789012345668901224567890aAbBcCdDeEfFgGhHiIjJkKlLmMnNoOpPq"
truenas_verify_ssl = false

# Resource configuration (must match 'create' values)
pool_name            = "testpool"
pool_disks           = ["sdb", "sdc", "sdd", "sde"]
nfs_allowed_network  = "192.168.1.0/24"
static_route_gateway = "192.168.1.1"
