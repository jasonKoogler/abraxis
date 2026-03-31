# Development Dockerfile
FROM golang:1.23-alpine

WORKDIR /app

# Install development tools
RUN apk add --no-cache git && \
    go install github.com/air-verse/air@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest


# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Expose ports for the application and delve
EXPOSE 8080 40000

# Use air for hot reloading
CMD ["air", "-c", ".air.toml"]