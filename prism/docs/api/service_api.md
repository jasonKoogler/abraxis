# Service API Documentation

The Service API provides a way to manage and interact with services and routes registered in the gateway.

## Authentication

Most Service API endpoints require authentication. Protected endpoints require a valid JWT token in the Authorization header:

```
Authorization: Bearer <your-token>
```

## Endpoints

### Service Management

#### List Services

Returns a list of all registered services.

- **URL**: `/api/services`
- **Method**: `GET`
- **Auth Required**: No
- **Permissions Required**: None

**Response**:

```json
{
  "services": [
    {
      "id": "user-service",
      "name": "User Service",
      "url": "http://user-service:8080",
      "health_status": "healthy",
      "last_checked": "2023-04-01T12:34:56Z"
    },
    {
      "id": "billing-service",
      "name": "Billing Service",
      "url": "http://billing-service:8080",
      "health_status": "unhealthy",
      "last_checked": "2023-04-01T12:34:56Z"
    }
  ]
}
```

#### Get Service Details

Returns details about a specific service.

- **URL**: `/api/services/{service_id}`
- **Method**: `GET`
- **Auth Required**: No
- **Permissions Required**: None

**Response**:

```json
{
  "id": "user-service",
  "name": "User Service",
  "url": "http://user-service:8080",
  "health_status": "healthy",
  "last_checked": "2023-04-01T12:34:56Z",
  "metrics": {
    "requests_total": 1234,
    "success_rate": 99.8,
    "average_response_time_ms": 45.2
  }
}
```

#### Register Service

Registers a new service with the gateway.

- **URL**: `/api/services`
- **Method**: `POST`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Request Body**:

```json
{
  "id": "new-service",
  "name": "New Service",
  "url": "http://new-service:8080",
  "health_check_path": "/health",
  "requires_auth": true,
  "allowed_methods": ["GET", "POST", "PUT", "DELETE"],
  "timeout": "30s",
  "retry_count": 3
}
```

**Response**:

```json
{
  "id": "new-service",
  "name": "New Service",
  "url": "http://new-service:8080",
  "status": "registered"
}
```

#### Update Service

Updates an existing service configuration.

- **URL**: `/api/services/{service_id}`
- **Method**: `PUT`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Request Body**:

```json
{
  "name": "Updated Service Name",
  "url": "http://updated-service:8080",
  "health_check_path": "/health",
  "requires_auth": true,
  "allowed_methods": ["GET", "POST", "PUT", "DELETE"],
  "timeout": "30s",
  "retry_count": 3
}
```

**Response**:

```json
{
  "id": "service-id",
  "name": "Updated Service Name",
  "url": "http://updated-service:8080",
  "status": "updated"
}
```

#### Deregister Service

Removes a service from the gateway.

- **URL**: `/api/services/{service_id}`
- **Method**: `DELETE`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Response**:

```json
{
  "id": "service-id",
  "status": "deregistered"
}
```

### Route Management

#### List All Routes

Returns a list of all registered routes across all services.

- **URL**: `/api/routes`
- **Method**: `GET`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Response**:

```json
{
  "routes": [
    {
      "path": "/api/users",
      "method": "GET",
      "service_id": "user-service",
      "service_name": "User Service",
      "public": false,
      "required_scopes": ["read:users"],
      "priority": 10
    },
    {
      "path": "/api/products",
      "method": "GET",
      "service_id": "product-service",
      "service_name": "Product Service",
      "public": true,
      "required_scopes": [],
      "priority": 10
    }
  ]
}
```

#### List Service Routes

Returns all routes registered for a specific service.

- **URL**: `/api/routes/services/{service_id}`
- **Method**: `GET`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Response**:

```json
{
  "service_id": "user-service",
  "service_name": "User Service",
  "routes": [
    {
      "path": "/api/users",
      "method": "GET",
      "public": false,
      "required_scopes": ["read:users"],
      "priority": 10
    },
    {
      "path": "/api/users/{id}",
      "method": "GET",
      "public": false,
      "required_scopes": ["read:users"],
      "priority": 10
    }
  ]
}
```

#### Register Route

Registers a new route for a service.

- **URL**: `/api/routes`
- **Method**: `POST`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Request Body**:

```json
{
  "service_id": "user-service",
  "path": "/api/users/profiles",
  "method": "GET",
  "public": false,
  "required_scopes": ["read:users", "read:profiles"],
  "priority": 10
}
```

**Response**:

```json
{
  "service_id": "user-service",
  "path": "/api/users/profiles",
  "method": "GET",
  "public": false,
  "required_scopes": ["read:users", "read:profiles"],
  "priority": 10,
  "status": "registered"
}
```

#### Update Route

Updates an existing route configuration.

- **URL**: `/api/routes/{route_id}`
- **Method**: `PUT`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Request Body**:

```json
{
  "public": true,
  "required_scopes": [],
  "priority": 20
}
```

**Response**:

```json
{
  "service_id": "user-service",
  "path": "/api/users/profiles",
  "method": "GET",
  "public": true,
  "required_scopes": [],
  "priority": 20,
  "status": "updated"
}
```

#### Delete Route

Removes a route from the gateway.

- **URL**: `/api/routes/{route_id}`
- **Method**: `DELETE`
- **Auth Required**: Yes
- **Permissions Required**: `service:admin`

**Response**:

```json
{
  "service_id": "user-service",
  "path": "/api/users/profiles",
  "method": "GET",
  "status": "deleted"
}
```

## Health Status

#### Service Health Check

Returns the health status of all registered services.

- **URL**: `/api/services-health`
- **Method**: `GET`
- **Auth Required**: No
- **Permissions Required**: None

**Response**:

```json
{
  "status": "healthy",
  "services": [
    {
      "id": "user-service",
      "status": "healthy",
      "last_checked": "2023-04-01T12:34:56Z",
      "response_time_ms": 42
    },
    {
      "id": "billing-service",
      "status": "unhealthy",
      "last_checked": "2023-04-01T12:34:56Z",
      "error": "Connection timeout"
    }
  ]
}
```

## Error Responses

The API uses standard HTTP status codes to indicate the success or failure of requests:

- `200 OK`: The request was successful
- `201 Created`: The resource was created successfully
- `400 Bad Request`: The request was invalid
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: The user does not have sufficient permissions
- `404 Not Found`: The requested resource was not found
- `500 Internal Server Error`: An error occurred on the server

Error responses have the following format:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The request was invalid",
    "details": "Service ID cannot be empty"
  }
}
```
