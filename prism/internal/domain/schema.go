package domain

import (
	"time"
)

// SchemaType identifies the type of schema
type SchemaType string

const (
	// SchemaTypeProtobuf is a Protocol Buffer schema
	SchemaTypeProtobuf SchemaType = "protobuf"
	// SchemaTypeOpenAPI is an OpenAPI schema
	SchemaTypeOpenAPI SchemaType = "openapi"
	// SchemaTypeGraphQL is a GraphQL schema
	SchemaTypeGraphQL SchemaType = "graphql"
)

// Schema represents a single API schema file
type Schema struct {
	ID           string      `json:"id" bson:"_id"`
	ServiceName  string      `json:"serviceName" bson:"serviceName"`
	Name         string      `json:"name" bson:"name"`
	Version      string      `json:"version" bson:"version"`
	SchemaType   SchemaType  `json:"schemaType" bson:"schemaType"`
	Content      []byte      `json:"content" bson:"content"`
	Dependencies []SchemaDep `json:"dependencies" bson:"dependencies"`
	Metadata     Metadata    `json:"metadata" bson:"metadata"`
	CreatedAt    time.Time   `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt" bson:"updatedAt"`
}

// SchemaDep represents a schema dependency
type SchemaDep struct {
	ServiceName string `json:"serviceName" bson:"serviceName"`
	Name        string `json:"name" bson:"name"`
	Version     string `json:"version" bson:"version"`
	Required    bool   `json:"required" bson:"required"`
}

// Metadata contains additional schema information
type Metadata struct {
	Author        string            `json:"author" bson:"author"`
	Description   string            `json:"description" bson:"description"`
	Labels        map[string]string `json:"labels" bson:"labels"`
	IsDeprecated  bool              `json:"isDeprecated" bson:"isDeprecated"`
	SuccessorName string            `json:"successorName" bson:"successorName"`
}

// SchemaBundle represents a collection of related schemas
type SchemaBundle struct {
	ID          string    `json:"id" bson:"_id"`
	ServiceName string    `json:"serviceName" bson:"serviceName"`
	Version     string    `json:"version" bson:"version"`
	SchemaIDs   []string  `json:"schemaIds" bson:"schemaIds"`
	IsComplete  bool      `json:"isComplete" bson:"isComplete"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" bson:"updatedAt"`
}

// ServiceRegistration represents a registered service instance
type ServiceRegistration struct {
	ID            string            `json:"id" bson:"_id"`
	ServiceName   string            `json:"serviceName" bson:"serviceName"`
	Version       string            `json:"version" bson:"version"`
	SchemaVersion string            `json:"schemaVersion" bson:"schemaVersion"`
	Address       string            `json:"address" bson:"address"`
	Port          int               `json:"port" bson:"port"`
	Metadata      map[string]string `json:"metadata" bson:"metadata"`
	Status        string            `json:"status" bson:"status"`
	LastHeartbeat time.Time         `json:"lastHeartbeat" bson:"lastHeartbeat"`
	RegisteredAt  time.Time         `json:"registeredAt" bson:"registeredAt"`
}

// SchemaCompatibility defines compatibility between schema versions
type SchemaCompatibility struct {
	ID                  string    `json:"id" bson:"_id"`
	ServiceName         string    `json:"serviceName" bson:"serviceName"`
	SchemaName          string    `json:"schemaName" bson:"schemaName"`
	OldVersion          string    `json:"oldVersion" bson:"oldVersion"`
	NewVersion          string    `json:"newVersion" bson:"newVersion"`
	IsCompatible        bool      `json:"isCompatible" bson:"isCompatible"`
	CompatibilityIssues []string  `json:"compatibilityIssues" bson:"compatibilityIssues"`
	CheckedAt           time.Time `json:"checkedAt" bson:"checkedAt"`
}

// SchemaEvent represents a schema-related event for notifications
type SchemaEvent struct {
	ID          string    `json:"id" bson:"_id"`
	EventType   string    `json:"eventType" bson:"eventType"`
	ServiceName string    `json:"serviceName" bson:"serviceName"`
	SchemaName  string    `json:"schemaName" bson:"schemaName"`
	Version     string    `json:"version" bson:"version"`
	Timestamp   time.Time `json:"timestamp" bson:"timestamp"`
	Details     string    `json:"details" bson:"details"`
}

// ServiceEvent represents a service-related event for notifications
type ServiceEvent struct {
	ID          string    `json:"id" bson:"_id"`
	EventType   string    `json:"eventType" bson:"eventType"`
	ServiceName string    `json:"serviceName" bson:"serviceName"`
	ServiceID   string    `json:"serviceId" bson:"serviceId"`
	Version     string    `json:"version" bson:"version"`
	Timestamp   time.Time `json:"timestamp" bson:"timestamp"`
	Details     string    `json:"details" bson:"details"`
}
