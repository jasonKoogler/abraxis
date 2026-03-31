package authz

###############################################################################
# Default: Deny unless all conditions are met.
###############################################################################
default allow = false

###############################################################################
# Superadmin bypass: Full access.
###############################################################################
allow {
  input.user.role == "superadmin"
}

###############################################################################
# Main authorization rule for non-superadmin users:
# - Tenant isolation must pass.
# - RBAC permissions must pass.
# - ABAC conditions (additional context) must pass.
###############################################################################
allow {
  tenant_isolation
  role_permission
  abac_conditions
}

###############################################################################
# Tenant Isolation: User and resource must belong to the same tenant.
###############################################################################
tenant_isolation {
  input.user.tenant_id == input.resource.tenant_id
}

###############################################################################
# Role-based Permission Check (RBAC):
#
# Permissions are defined externally (e.g., in data.permissions) with a structure like:
#
# {
#   "member": {
#     "GET":    { "business_owner": true, "manager": true, "employee": true },
#     "POST":   { "business_owner": true, "manager": true },
#     "PUT":    { "business_owner": true, "manager": true },
#     "DELETE": { "business_owner": true }
#   },
#   "booking": {
#     "GET":    { "business_owner": true, "manager": true, "employee": true },
#     "POST":   { "business_owner": true, "manager": true },
#     "PUT":    { "business_owner": true, "manager": true },
#     "DELETE": { "business_owner": true }
#   }
# }
###############################################################################
role_permission {
  perms := data.permissions[input.resource.type]
  perms != null
  method_perms := perms[input.method]
  method_perms != null
  method_perms[input.user.role] == true
}

###############################################################################
# ABAC Conditions: Additional attribute-based checks.
###############################################################################
abac_conditions {
  resource_active
  time_based_access
  ip_address_check
  device_check
  mfa_check
  # Add more ABAC conditions as needed.
}

###############################################################################
# ABAC Condition: The resource must be active.
###############################################################################
resource_active {
  input.resource.status == "active"
}

###############################################################################
# ABAC Condition: Time-based access.
#
# If the resource defines allowed time windows (e.g. allowed_from_time,
# allowed_to_time), then the current time (input.current_time) must fall within
# that window. If no times are specified, then this check passes by default.
#
# Note: current_time, allowed_from_time, and allowed_to_time are assumed to be
# Unix timestamps (seconds or milliseconds, but must be consistent).
###############################################################################
time_based_access {
  not input.resource.allowed_from_time
  not input.resource.allowed_to_time
}

time_based_access {
  current_time := input.current_time
  allowed_from := input.resource.allowed_from_time
  allowed_to := input.resource.allowed_to_time
  current_time >= allowed_from
  current_time <= allowed_to
}

###############################################################################
# ABAC Condition: IP Address Check.
#
# If the resource specifies allowed IP ranges (input.resource.allowed_ips as a
# list of CIDR strings), then the client's IP (input.client_ip) must fall within
# one of these ranges.
###############################################################################
ip_address_check {
  not input.resource.allowed_ips
}

ip_address_check {
  some i
  allowed_cidr := input.resource.allowed_ips[i]
  ip_in_range(input.client_ip, allowed_cidr)
}

###############################################################################
# ABAC Condition: Device Check.
#
# If the resource specifies allowed device types (input.resource.allowed_devices),
# then the client's device type (input.device.type) must be one of those.
###############################################################################
device_check {
  not input.resource.allowed_devices
}

device_check {
  some d
  allowed_device := input.resource.allowed_devices[d]
  allowed_device == input.device.type
}

###############################################################################
# ABAC Condition: Multi-Factor Authentication (MFA) Check.
#
# If the resource requires MFA (input.resource.mfa_required is true), then thef
# user must have a valid MFA status (input.user.mfa_valid must be true).
###############################################################################
mfa_check {
  not input.resource.mfa_required
}

mfa_check {
  input.resource.mfa_required == true
  input.user.mfa_valid == true
}

###############################################################################
# Helper Function: Check if a given client IP is within a CIDR range.
#
# This implementation uses OPA's built-in net.cidr_contains function to
# properly check if an IP address falls within a CIDR range.
###############################################################################
ip_in_range(client_ip, cidr) {
  net.cidr_contains(cidr, client_ip)
}
