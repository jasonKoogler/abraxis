package authz

###############################################################################
# Default: Deny unless all conditions are met.
###############################################################################
default allow = false

###############################################################################
# Role-based access control with tenant isolation
###############################################################################

# Allow access if the user has the required role for the resource
allow {
    # Tenant isolation: User and resource must belong to the same tenant
    input.user.tenant_id == input.resource.tenant_id
    
    # Role-based access control
    has_permission(input.user.role, input.resource.type, input.method)
}

# Define role-based permissions for different resource types and HTTP methods
has_permission(role, resource_type, method) {
    # Admin role has full access to all resources
    role == "admin"
}

has_permission(role, resource_type, method) {
    # Editor role can read and write but not delete
    role == "editor"
    method_permissions := editor_permissions[resource_type][method]
    method_permissions == true
}

has_permission(role, resource_type, method) {
    # Viewer role can only read
    role == "viewer"
    method_permissions := viewer_permissions[resource_type][method]
    method_permissions == true
}

# Define permissions for editor role
editor_permissions = {
    # Users resource
    "users": {
        "GET": true,
        "POST": true,
        "PUT": true,
        "DELETE": false
    },
    # Other resources
    "documents": {
        "GET": true,
        "POST": true,
        "PUT": true,
        "DELETE": false
    },
    "settings": {
        "GET": true,
        "POST": true,
        "PUT": true,
        "DELETE": false
    }
}

# Define permissions for viewer role
viewer_permissions = {
    # Users resource
    "users": {
        "GET": true,
        "POST": false,
        "PUT": false,
        "DELETE": false
    },
    # Other resources
    "documents": {
        "GET": true,
        "POST": false,
        "PUT": false,
        "DELETE": false
    },
    "settings": {
        "GET": true,
        "POST": false,
        "PUT": false,
        "DELETE": false
    }
}

###############################################################################
# Additional attribute-based access control (ABAC) rules
###############################################################################

# Allow access if the user is accessing their own user resource
allow {
    # Resource is a user
    input.resource.type == "users"
    
    # User is accessing their own resource
    input.resource.id == input.user.id
    
    # Only allow GET method for self-access
    input.method == "GET"
}

# Allow access if the user is accessing their own user resource with PUT
allow {
    # Resource is a user
    input.resource.type == "users"
    
    # User is accessing their own resource
    input.resource.id == input.user.id
    
    # Only allow PUT method for self-access
    input.method == "PUT"
}

# Allow access based on IP address restrictions (if defined)
allow {
    # Basic tenant isolation and role check
    input.user.tenant_id == input.resource.tenant_id
    has_permission(input.user.role, input.resource.type, input.method)
    
    # IP address check (if resource has allowed_ips)
    not input.resource.allowed_ips
}

allow {
    # Basic tenant isolation and role check
    input.user.tenant_id == input.resource.tenant_id
    has_permission(input.user.role, input.resource.type, input.method)
    
    # IP address check (if resource has allowed_ips)
    some i
    cidr := input.resource.allowed_ips[i]
    net.cidr_contains(cidr, input.client_ip)
}

###############################################################################
# Special case: Public endpoints that don't require authentication
###############################################################################

# Allow access to public endpoints without authentication
allow {
    is_public_endpoint(input.path)
}

# Define public endpoints
is_public_endpoint(path) {
    public_paths := [
        "/auth/login",
        "/auth/register",
        "/health",
        "/metrics"
    ]
    
    some i
    path == public_paths[i]
}

# Allow access to public endpoints with specific prefixes
is_public_endpoint(path) {
    startswith(path, "/auth/")
    public_prefixes := [
        "/auth/google",
        "/auth/github",
        "/auth/facebook"
    ]
    
    some i
    startswith(path, public_prefixes[i])
} 