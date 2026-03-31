package circuitbreaker

// serviceNameContextKey is a context key for the service name
type serviceNameContextKey string

// ServiceNameContextKey is the context key for the service name
// mostly used for testing
const ServiceNameContextKey = serviceNameContextKey("serviceName")
