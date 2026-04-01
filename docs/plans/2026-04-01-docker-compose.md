# Docker Compose Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Root-level `docker compose up` starts both services with Postgres, Redis, auto-migrations, hot reload, and debug ports.

**Architecture:** Single docker-compose.yml at monorepo root with shared Postgres (two databases) and Redis. Dev Dockerfiles use air for hot reload. Migration containers run once on startup then exit.

**Tech Stack:** Docker Compose, golang:1.24-alpine, postgres:16-alpine, redis:7-alpine, migrate/migrate, air, delve

**Spec:** `docs/specs/2026-04-01-docker-compose-design.md`

---

### Task 1: Create Dockerfiles and init script

**Files:**
- Create: `/home/jason/jdk/abraxis/docker/aegis.Dockerfile`
- Create: `/home/jason/jdk/abraxis/docker/prism.Dockerfile`
- Create: `/home/jason/jdk/abraxis/docker/init-db.sql`

- [ ] **Step 1: Create the docker/ directory**

```bash
mkdir -p /home/jason/jdk/abraxis/docker
```

- [ ] **Step 2: Create aegis.Dockerfile**

Create `/home/jason/jdk/abraxis/docker/aegis.Dockerfile`:

```dockerfile
FROM golang:1.24-alpine

WORKDIR /app

RUN apk add --no-cache git wget && \
    go install github.com/air-verse/air@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080 9090 40000

CMD ["air", "-c", ".air.toml"]
```

- [ ] **Step 3: Create prism.Dockerfile**

Create `/home/jason/jdk/abraxis/docker/prism.Dockerfile`:

```dockerfile
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
```

- [ ] **Step 4: Create init-db.sql**

Create `/home/jason/jdk/abraxis/docker/init-db.sql`:

```sql
-- Create separate databases for each service.
-- The default 'postgres' database is created automatically.
CREATE DATABASE aegis_db;
CREATE DATABASE prism_db;
```

- [ ] **Step 5: Commit**

```bash
cd /home/jason/jdk/abraxis
git add docker/
git commit -m "feat: add dev Dockerfiles and Postgres init script"
```

---

### Task 2: Create docker-compose.yml

**Files:**
- Create: `/home/jason/jdk/abraxis/docker-compose.yml`

- [ ] **Step 1: Create docker-compose.yml**

Create `/home/jason/jdk/abraxis/docker-compose.yml`:

```yaml
services:
  # ---------------------------------------------------------------------------
  # Infrastructure
  # ---------------------------------------------------------------------------
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - pg-data:/var/lib/postgresql/data
      - ./docker/init-db.sql:/docker-entrypoint-initdb.d/init-db.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "redis", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10
    restart: unless-stopped

  # ---------------------------------------------------------------------------
  # Migrations (run once, then exit)
  # ---------------------------------------------------------------------------
  migrate-aegis:
    image: migrate/migrate
    command:
      - -path=/migrations
      - -database=postgres://postgres:postgres@postgres:5432/aegis_db?sslmode=disable
      - up
    volumes:
      - ./aegis/deploy/migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy

  migrate-prism:
    image: migrate/migrate
    command:
      - -path=/migrations
      - -database=postgres://postgres:postgres@postgres:5432/prism_db?sslmode=disable
      - up
    volumes:
      - ./prism/deploy/migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy

  # ---------------------------------------------------------------------------
  # Services
  # ---------------------------------------------------------------------------
  aegis:
    build:
      context: ./aegis
      dockerfile: ../docker/aegis.Dockerfile
    ports:
      - "8080:8080"   # HTTP
      - "9090:9090"   # gRPC
      - "40000:40000" # Delve debugger
    environment:
      # App
      ENV: development
      LOG_LEVEL: debug
      # Postgres
      POSTGRES_HOST: postgres
      POSTGRES_PORT: "5432"
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: aegis_db
      POSTGRES_SSL_MODE: disable
      POSTGRES_TIMEZONE: UTC
      POSTGRES_TIMEOUT: 30s
      # Redis
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      REDIS_PASSWORD: redis
      REDIS_USERNAME: default
      # HTTP
      HTTP_SERVER_PORT: "8080"
      HTTP_SERVER_READ_TIMEOUT: 10s
      HTTP_SERVER_WRITE_TIMEOUT: 10s
      HTTP_SERVER_IDLE_TIMEOUT: 10s
      HTTP_SERVER_SHUTDOWN_TIMEOUT: 10s
      CORS_ALLOWED_ORIGINS: "http://localhost:3000"
      # Rate Limiting
      USE_REDIS_RATE_LIMITER: "true"
      RATE_LIMIT_REQUESTS_PER_SECOND: "100"
      RATE_LIMIT_BURST: "150"
      RATE_LIMIT_TTL: 1h
      # JWT
      JWT_ISSUER: aegis
      # OAuth (placeholder — replace with real values)
      GOOGLE_KEY: placeholder
      GOOGLE_SECRET: placeholder
      GOOGLE_CALLBACK_URL: http://localhost:8080/auth/google/callback
      GOOGLE_SCOPES: "https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/userinfo.profile"
      FACEBOOK_KEY: placeholder
      FACEBOOK_SECRET: placeholder
      FACEBOOK_CALLBACK_URL: http://localhost:8080/auth/facebook/callback
      FACEBOOK_SCOPES: email
      TWITTER_KEY: placeholder
      TWITTER_SECRET: placeholder
      TWITTER_CALLBACK_URL: http://localhost:8080/auth/twitter/callback
      TWITTER_SCOPES: "tweet.read,users.read"
      OAUTH_VERIFIER_STORAGE: redis
      # gRPC
      GRPC_PORT: "9090"
      GRPC_ENABLED: "true"
    volumes:
      - ./aegis:/app
      - aegis-go-cache:/root/go/pkg/mod
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      migrate-aegis:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 10
      start_period: 30s
    restart: unless-stopped

  prism:
    build:
      context: ./prism
      dockerfile: ../docker/prism.Dockerfile
    ports:
      - "8081:8080"   # HTTP (remapped to 8081 on host)
      - "40001:40000" # Delve debugger
    environment:
      # App
      ENV: development
      LOG_LEVEL: debug
      # Postgres
      POSTGRES_HOST: postgres
      POSTGRES_PORT: "5432"
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: prism_db
      POSTGRES_SSL_MODE: disable
      POSTGRES_TIMEZONE: UTC
      POSTGRES_TIMEOUT: 30s
      # Redis
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      REDIS_PASSWORD: redis
      REDIS_USERNAME: default
      # HTTP
      HTTP_SERVER_PORT: "8080"
      HTTP_SERVER_READ_TIMEOUT: 10s
      HTTP_SERVER_WRITE_TIMEOUT: 10s
      HTTP_SERVER_IDLE_TIMEOUT: 10s
      HTTP_SERVER_SHUTDOWN_TIMEOUT: 10s
      CORS_ALLOWED_ORIGINS: "http://localhost:3000"
      # Rate Limiting
      USE_REDIS_RATE_LIMITER: "true"
      RATE_LIMIT_REQUESTS_PER_SECOND: "100"
      RATE_LIMIT_BURST: "150"
      RATE_LIMIT_TTL: 1h
      # JWT
      JWT_ISSUER: aegis
      # OAuth (placeholder — replace with real values)
      GOOGLE_KEY: placeholder
      GOOGLE_SECRET: placeholder
      GOOGLE_CALLBACK_URL: http://localhost:8080/auth/google/callback
      GOOGLE_SCOPES: "https://www.googleapis.com/auth/userinfo.email,https://www.googleapis.com/auth/userinfo.profile"
      FACEBOOK_KEY: placeholder
      FACEBOOK_SECRET: placeholder
      FACEBOOK_CALLBACK_URL: http://localhost:8080/auth/facebook/callback
      FACEBOOK_SCOPES: email
      TWITTER_KEY: placeholder
      TWITTER_SECRET: placeholder
      TWITTER_CALLBACK_URL: http://localhost:8080/auth/twitter/callback
      TWITTER_SCOPES: "tweet.read,users.read"
      OAUTH_VERIFIER_STORAGE: redis
      # Aegis integration
      AEGIS_GRPC_ADDRESS: aegis:9090
      AEGIS_SYNC_ENABLED: "true"
      AEGIS_CACHE_TTL: 60s
      AEGIS_RECONNECT_MAX_BACKOFF: 30s
      AEGIS_JWKS_URL: http://aegis:8080/.well-known/jwks.json
      AEGIS_JWKS_REFRESH_INTERVAL: 5m
    volumes:
      - ./prism:/app
      - prism-go-cache:/root/go/pkg/mod
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      migrate-prism:
        condition: service_completed_successfully
      aegis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 10
      start_period: 30s
    restart: unless-stopped

volumes:
  pg-data:
  redis-data:
  aegis-go-cache:
  prism-go-cache:
```

- [ ] **Step 2: Verify compose config parses**

```bash
cd /home/jason/jdk/abraxis
docker compose config --quiet
```

Expected: no output (silent success). If errors, fix YAML syntax.

- [ ] **Step 3: Commit**

```bash
cd /home/jason/jdk/abraxis
git add docker-compose.yml
git commit -m "feat: add root docker-compose.yml with full dev stack"
```

---

### Task 3: Test the full stack

- [ ] **Step 1: Build and start all services**

```bash
cd /home/jason/jdk/abraxis
docker compose up --build -d
```

- [ ] **Step 2: Check container status**

```bash
docker compose ps
```

Expected: postgres, redis running. migrate-aegis, migrate-prism exited (0). aegis, prism running or starting.

- [ ] **Step 3: Wait for health checks and verify**

```bash
# Wait for services to be healthy
sleep 30
docker compose ps
```

Expected: aegis and prism show "healthy" status.

- [ ] **Step 4: Test endpoints**

```bash
# Aegis health
curl -s http://localhost:8080/health

# Aegis swagger
curl -s http://localhost:8080/swagger/index.html | head -5

# Prism health (port 8081)
curl -s http://localhost:8081/health

# Prism swagger
curl -s http://localhost:8081/swagger/index.html | head -5

# Aegis gRPC port is open
nc -z localhost 9090 && echo "gRPC port open" || echo "gRPC port closed"
```

- [ ] **Step 5: Check logs for errors**

```bash
docker compose logs aegis --tail 20
docker compose logs prism --tail 20
```

Look for startup errors, connection failures, or panics.

- [ ] **Step 6: Test hot reload (optional manual test)**

Edit a handler file (e.g., add a comment), watch air rebuild in the logs:

```bash
docker compose logs -f aegis
# In another terminal: edit a .go file in aegis/
```

- [ ] **Step 7: Clean up if tests pass**

```bash
docker compose down
```

- [ ] **Step 8: Fix any issues found during testing, then commit**

If any fixes were needed to Dockerfiles, compose, or init script:

```bash
cd /home/jason/jdk/abraxis
git add -A
git commit -m "fix: docker compose adjustments from testing"
```

- [ ] **Step 9: Push**

```bash
cd /home/jason/jdk/abraxis
git push origin main
```
