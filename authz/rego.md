# Understanding OPA Policy Organization and Evaluation

Let me thoroughly explain how the import system works in OPA policies and how query paths affect policy evaluation.

## Imports in Rego Files

When you see `import data.rbac` or `import data.abac` in a Rego file, this is similar to imports in programming languages, but with some key differences:

### What Imports Do in Rego

1. **Access to Another Package's Data**: The import allows one package to access rules, functions, and constants defined in another package.

2. **Reference Format**: After importing `data.rbac`, you can reference rules from that package using `rbac.rule_name`.

3. **Data Organization**: All policy data in OPA lives under the `data` document tree. The import statement is just a way to create a local reference to a branch of that tree.

### Example:

When you write:
```rego
package authz

import data.rbac
import data.abac

allow {
    rbac.allow
}
```

You're saying:
- Define rules in the `authz` package
- Create a local reference `rbac` that points to `data.rbac`
- Create a local reference `abac` that points to `data.abac`
- Define a rule `allow` that is true if `data.rbac.allow` is true

## Package System in Rego

Each Rego file declares its package with `package <name>`:

1. **Namespace**: The package creates a namespace for rules.

2. **Data Structure**: When loaded into OPA, all rules in `package rbac` will be accessible at `data.rbac`.

3. **Multiple Files, Same Package**: Multiple files can contribute to the same package. If two files declare `package rbac`, their rules will be merged under `data.rbac`.

4. **No Conflicts**: Rules in different packages never conflict, even if they have the same name.

## How `WithDefaultQuery` Works

When you call `authz.WithDefaultQuery("data.payments.allow")`, you're telling the Authz library:

1. **Evaluation Target**: "When I call `Evaluate()`, assess whether `data.payments.allow` is true for the provided input."

2. **Package Selection**: This selects which package's rule to evaluate (in this case, the `allow` rule in the `payments` package).

3. **Query Path Format**: The path follows the format `data.<package>.<rule>`.

## Complete Evaluation Process

Here's what happens in full when you evaluate with `data.payments.allow`:

1. **Policy Loading**: All your Rego files are loaded and compiled by OPA.

2. **Rule Organization**: Rules are organized by package into OPA's internal `data` document:
   - `package rbac` rules go to `data.rbac`
   - `package abac` rules go to `data.abac`
   - `package payments` rules go to `data.payments`
   - etc.

3. **Evaluation Request**: When you call `Evaluate()`, it builds a query to OPA for `data.payments.allow`.

4. **Rule Chain Execution**: OPA evaluates `data.payments.allow`:
   ```rego
   # From payments.rego
   allow {
       authz.allow  # This references data.authz.allow
   }
   
   allow {
       input.action == "process_payment"
       # other conditions...
   }
   ```

5. **Import Resolution**: When evaluating `authz.allow`, OPA follows the reference to `data.authz.allow`:
   ```rego
   # From main.rego (package authz)
   allow {
       rbac.allow  # This references data.rbac.allow
   }
   
   allow {
       abac.allow  # This references data.abac.allow
   }
   ```

6. **Recursive Evaluation**: OPA continues following references, evaluating rules in other packages:
   - It checks `data.rbac.allow` from the rbac.rego file
   - It checks `data.abac.allow` from the abac.rego file
   - These might reference other rules recursively

7. **Result Aggregation**: A rule is true if any of its definitions evaluate to true. So `data.payments.allow` is true if any of its defined conditions are met.

## Practical Impact on Your Authorization Design

This system allows you to:

1. **Create Service-Specific Rules**: Each service can have its own package (e.g., `payments`, `users`, `reports`).

2. **Reuse Common Logic**: Common authorization patterns can be defined once and imported.

3. **Layer Authorization Models**: Use RBAC as your baseline and add ABAC for exceptions.

4. **Switch Contexts**: Different endpoints might use different evaluation paths:
   - `authz.WithDefaultQuery("data.payments.allow")` for payment endpoints
   - `authz.WithDefaultQuery("data.documents.allow")` for document endpoints

5. **Override Rules**: Service-specific packages can override or extend general rules.

## Complete Real-World Example

Imagine a payments microservice that needs to authorize API requests. You could configure it like:

```go
policies := map[string]string{
    "main.rego": `
        package authz
        
        import data.rbac
        import data.abac
        
        default allow = false
        
        # Core logic that combines RBAC and ABAC
        allow {
            rbac.allow
        }
        
        allow {
            abac.allow
        }
    `,
    "rbac.rego": `
        package rbac
        
        default allow = false
        
        # Role-based permissions
        allow {
            role := input.user.roles[_]
            permissions[role][input.action][input.resource.type]
        }
        
        permissions = {
            "payment_admin": {
                "read": {"payment": true, "refund": true},
                "write": {"payment": true, "refund": true},
                "approve": {"payment": true, "refund": true}
            },
            "payment_processor": {
                "read": {"payment": true},
                "write": {"payment": true},
                "approve": {"payment": false}
            }
        }
    `,
    "payments.rego": `
        package payments
        
        import data.authz
        
        default allow = false
        
        # First try general authorization
        allow {
            authz.allow
        }
        
        # Payment-specific logic - amount thresholds
        allow {
            # Payment processors can only handle payments under $10,000
            input.action == "approve"
            input.user.roles[_] == "payment_processor"
            input.resource.type == "payment"
            input.resource.amount < 10000
        }
        
        # Special case: creators can't approve their own payments
        allow = false {
            input.action == "approve"
            input.resource.type == "payment"
            input.resource.created_by == input.user.id
        }
    `,
}

// Create the authz instance for the payments service
authzService, err := authz.New(
    authz.WithLocalPolicies(policies),
    authz.WithDefaultQuery("data.payments.allow"),  // Use payment-specific rules
    authz.WithMemoryCache(time.Minute*5, 1000),
)
```

With this configuration:

1. The payments service uses `data.payments.allow` as its entry point.
2. This first checks general authorization via `authz.allow`.
3. It then applies payment-specific rules like amount thresholds.
4. It includes a special denial rule for self-approvals.
5. The rule evaluation forms a tree, starting at `data.payments.allow` and potentially traversing to `data.authz.allow`, `data.rbac.allow`, and `data.abac.allow`.

When you call `authzService.Evaluate(ctx, input)`, it follows this entire chain to determine if access should be allowed.

This hierarchical approach provides both consistency (core rules) and flexibility (service-specific overrides) in your authorization system.




# Route-Specific Authorization Rules in OPA

You can easily implement route-specific authorization rules within your policy structure. This allows you to define precise access controls for individual API endpoints or routes. Here's how to approach this:

## Integrating Route-Specific Rules

There are several ways to handle route-specific authorization:

### Approach 1: Route Information in Input

The simplest approach is to include route information in your input object and evaluate it in your policies:

```rego
package api

default allow = false

# Allow access to specific routes based on roles
allow {
    # GET /users - accessible to users with admin or user_manager roles
    input.http.method == "GET"
    input.http.path == "/users"
    input.user.roles[_] == "admin"
}

allow {
    input.http.method == "GET"
    input.http.path == "/users"
    input.user.roles[_] == "user_manager"
}

# POST /users - only admins can create users
allow {
    input.http.method == "POST"
    input.http.path == "/users"
    input.user.roles[_] == "admin"
}

# GET /users/{id} - users can access their own data, admins can access any
allow {
    input.http.method == "GET"
    regex.match("/users/[^/]+", input.http.path)
    input.user.roles[_] == "admin"
}

allow {
    input.http.method == "GET"
    input.http.path == concat("/users/", [input.user.id])
    input.user.roles[_] == "user"
}
```

### Approach 2: Organize by Route Path

For larger APIs, you might want to organize your policies by route or API section:

```rego
package routes

# Users API
users_get {
    input.user.roles[_] == "admin"
}

users_get {
    input.user.roles[_] == "user_manager"
}

users_create {
    input.user.roles[_] == "admin"
}

users_get_by_id {
    # Admins can see any user
    input.user.roles[_] == "admin"
}

users_get_by_id {
    # Users can see their own profile
    path_id := trim_prefix(input.http.path, "/users/")
    path_id == input.user.id
}

# Orders API
orders_list {
    input.user.roles[_] == "admin"
}

orders_list {
    input.user.roles[_] == "order_manager"
}

# Main allow rule that dispatches to the appropriate route handler
default allow = false

allow {
    input.http.method == "GET"
    input.http.path == "/users"
    users_get
}

allow {
    input.http.method == "POST"
    input.http.path == "/users"
    users_create
}

allow {
    input.http.method == "GET"
    regex.match("/users/[^/]+", input.http.path)
    users_get_by_id
}

allow {
    input.http.method == "GET"
    input.http.path == "/orders"
    orders_list
}
```

### Approach 3: Path-Based Package Structure

For very large APIs, you can organize with separate packages for different parts of your API:

```rego
# users.rego
package api.users

default allow = false

# GET /users
allow {
    input.http.method == "GET"
    input.http.path == "/users"
    can_list_users
}

# GET /users/{id}
allow {
    input.http.method == "GET"
    regex.match("/users/[^/]+", input.http.path)
    can_view_user
}

# Helper rules
can_list_users {
    input.user.roles[_] == "admin"
}

can_list_users {
    input.user.roles[_] == "user_manager"
}

can_view_user {
    input.user.roles[_] == "admin"
}

can_view_user {
    path_id := trim_prefix(input.http.path, "/users/")
    path_id == input.user.id
}
```

```rego
# orders.rego
package api.orders

default allow = false

# GET /orders
allow {
    input.http.method == "GET"
    input.http.path == "/orders"
    can_list_orders
}

# Helper rules
can_list_orders {
    input.user.roles[_] == "admin"
}

can_list_orders {
    input.user.roles[_] == "order_manager"
}
```

```rego
# main.rego
package api

import data.api.users
import data.api.orders

default allow = false

# Routing logic
allow {
    startswith(input.http.path, "/users")
    users.allow
}

allow {
    startswith(input.http.path, "/orders")
    orders.allow
}
```

## Integration with Your HTTP Middleware

To use route-specific rules, modify your input extraction function:

```go
func extractInput(r *http.Request) (interface{}, error) {
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        return nil, fmt.Errorf("missing user ID")
    }
    
    // Get roles from header or JWT
    roles := []string{}
    // ... extract roles ...
    
    // Build input object with HTTP details
    input := map[string]interface{}{
        "user": map[string]interface{}{
            "id":    userID,
            "roles": roles,
        },
        "http": map[string]interface{}{
            "method": r.Method,
            "path":   r.URL.Path,
            "query":  r.URL.Query(),
            "headers": map[string]interface{}{
                "content_type": r.Header.Get("Content-Type"),
                // Add other relevant headers
            },
        },
        "resource": map[string]interface{}{
            "type": getResourceTypeFromPath(r.URL.Path),
            // Other resource attributes
        },
    }
    
    return input, nil
}
```

Then configure your Authz instance with the appropriate query path:

```go
authzService, err := authz.New(
    authz.WithLocalPolicies(policies),
    authz.WithDefaultQuery("data.api.allow"),  // Path to your main API allow rule
    // Other options...
)
```

## Additional Advanced Techniques

### 1. URL Path Parameter Extraction

For RESTful APIs with path parameters:

```rego
# Extract user ID from path like /users/{id}
path_components := split(input.http.path, "/")
user_id := path_components[2]  # /users/123 -> ["", "users", "123"]

# Check if requesting user has access to target user
allow {
    input.http.method == "GET"
    path_components := split(input.http.path, "/")
    path_components[1] == "users"  # Path starts with /users/
    target_id := path_components[2]
    
    # Admin can access any user
    input.user.roles[_] == "admin"
}

allow {
    input.http.method == "GET"
    path_components := split(input.http.path, "/")
    path_components[1] == "users"  # Path starts with /users/
    target_id := path_components[2]
    
    # Users can access their own data
    input.user.id == target_id
}
```

### 2. Complex Route-Specific Logic

For routes with complex authorization requirements:

```rego
# PUT /organizations/{org_id}/teams/{team_id}/members/{user_id}
allow {
    input.http.method == "PUT"
    path_components := split(input.http.path, "/")
    path_components[1] == "organizations"
    org_id := path_components[2]
    path_components[3] == "teams"
    team_id := path_components[4]
    path_components[5] == "members"
    
    # Check organizational membership
    input.user.organizations[org_id]
    
    # Check team admin status
    input.user.team_roles[team_id] == "admin"
}
```

### 3. Regex-Based Path Matching

For complex path patterns:

```rego
allow {
    input.http.method == "GET"
    
    # Match paths like /api/v1/reports/monthly/2023/01
    regex.match("/api/v1/reports/monthly/[0-9]{4}/[0-9]{2}", input.http.path)
    
    # User has report viewer role
    input.user.roles[_] == "report_viewer"
}
```

## Choosing Your Approach

When implementing route-specific authorization:

1. **Simple APIs**: Use Approach 1 with direct path checking in your rules.
   
2. **Medium-sized APIs**: Use Approach 2 with rule organization by endpoint function.
   
3. **Large/Complex APIs**: Use Approach 3 with package-based organization.

The key is to structure your policies in a way that balances:
- Readability and maintainability
- Performance (fewer rules to evaluate)
- Flexibility for future changes

Would you like me to elaborate on any specific aspect of route-based authorization or provide examples for specific use cases?