FROM golang:1.24-alpine

WORKDIR /app

RUN apk add --no-cache git wget && \
    go install github.com/air-verse/air@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080 40000

CMD ["air", "-c", ".air.toml"]
