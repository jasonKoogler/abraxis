FROM golang:1.24-alpine

WORKDIR /workspace

RUN apk add --no-cache git wget && \
    go install github.com/air-verse/air@v1.61.7 && \
    go install github.com/go-delve/delve/cmd/dlv@latest

# Source code is mounted as a volume in docker-compose.
# Go module cache is a named volume for persistence.
EXPOSE 8080 40000

CMD ["air", "-c", ".air.toml"]
