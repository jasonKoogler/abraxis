# Monorepo Consolidation Design Spec

**Date:** 2026-03-30
**Scope:** Consolidate Aegis, Prism, and Authz into a single monorepo called Abraxis

## Motivation

The three repos (Aegis, Prism, Authz) are tightly coupled: both services depend on authz, Prism uses Aegis's proto via workspace, and `go mod tidy` fails without workspace mode. Separate repos create friction (coordinated commits, go.work hacks, GOWORK=off for swag init) without real benefit — they're a solo project intended as a cohesive platform.

## Target Structure

**Repo:** `github.com/jasonKoogler/abraxis`

```
abraxis/
├── aegis/                # auth service
│   ├── cmd/
│   ├── internal/
│   ├── api/grpc/
│   ├── docs/
│   ├── policies/
│   ├── deploy/
│   ├── Makefile
│   └── go.mod            # github.com/jasonKoogler/abraxis/aegis
├── prism/                # API gateway
│   ├── cmd/
│   ├── internal/
│   ├── middleware/
│   ├── docs/
│   ├── policies/
│   ├── deploy/
│   ├── Makefile
│   └── go.mod            # github.com/jasonKoogler/abraxis/prism
├── authz/                # shared OPA library
│   ├── examples/
│   └── go.mod            # github.com/jasonKoogler/abraxis/authz
├── go.work               # use ./aegis ./prism ./authz
├── docker-compose.yml    # both services + Postgres + Redis
├── Makefile              # root orchestration
└── README.md
```

## Module Strategy

Multi-module monorepo. Each service/library keeps its own `go.mod` with updated module paths:

| Current | New |
|---------|-----|
| `github.com/jasonKoogler/aegis` | `github.com/jasonKoogler/abraxis/aegis` |
| `github.com/jasonKoogler/prism` | `github.com/jasonKoogler/abraxis/prism` |
| `github.com/jasonKoogler/authz` | `github.com/jasonKoogler/abraxis/authz` |

A root `go.work` ties them together for local development.

## Git History

Fresh start. Copy current state into the new structure with a single initial commit. Old repos stay archived for reference.

## Mechanical Changes Required

### 1. Create repo and copy files

- Create `github.com/jasonKoogler/abraxis` on GitHub
- Create local directory structure
- Copy all files from each repo (excluding `.git/`, `go.work`, `go.work.sum`)

### 2. Update module paths in go.mod

Each `go.mod` gets a new module path and updated internal dependency references:

**aegis/go.mod:**
- Module: `github.com/jasonKoogler/abraxis/aegis`
- `github.com/jasonKoogler/authz` → `github.com/jasonKoogler/abraxis/authz`

**prism/go.mod:**
- Module: `github.com/jasonKoogler/abraxis/prism`
- `github.com/jasonKoogler/authz` → `github.com/jasonKoogler/abraxis/authz`

**authz/go.mod:**
- Module: `github.com/jasonKoogler/abraxis/authz`
- No internal dep changes (standalone)

### 3. Find-and-replace import paths in all Go files

Three replacements across all `.go` files:
- `github.com/jasonKoogler/aegis` → `github.com/jasonKoogler/abraxis/aegis`
- `github.com/jasonKoogler/prism` → `github.com/jasonKoogler/abraxis/prism`
- `github.com/jasonKoogler/authz` → `github.com/jasonKoogler/abraxis/authz`

This affects:
- All Go source files (`internal/`, `cmd/`, etc.)
- Generated proto files (`api/grpc/aegispb/*.go`)
- Generated swagger docs (`docs/docs.go`)

### 4. Regenerate proto

The proto `go_package` option references the old module path. Update `aegis/api/grpc/aegispb/aegis.proto` and regenerate:

```
option go_package = "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb";
```

Then run `make proto` in aegis.

### 5. Regenerate swagger

Import path changes affect swagger type resolution. Run `make swagger` in both aegis and prism.

### 6. Create root go.work

```
go 1.24.0

use (
    ./aegis
    ./prism
    ./authz
)
```

### 7. Create root Makefile

Thin orchestrator that delegates to service Makefiles:

```makefile
.PHONY: build-all test-all swagger-all

build-all:
	$(MAKE) -C aegis build
	$(MAKE) -C prism build

test-all:
	$(MAKE) -C aegis test
	$(MAKE) -C prism test

swagger-all:
	$(MAKE) -C aegis swagger
	$(MAKE) -C prism swagger
```

### 8. Update service Makefiles

- Proto generation scripts may reference old module paths — update `scripts/protogen.sh` in aegis
- Swagger `GOWORK=off`: test whether it's still needed. The cross-module annotation leakage issue (swag following imports into other workspace modules) may persist in the monorepo. Keep `GOWORK=off` if removing it causes other services' annotations to leak into the swagger output.

### 9. Clean go.sum files

Run `go mod tidy` for each module in workspace mode. This should now work since all modules resolve locally via go.work. This cleans stale entries and properly declares cross-module dependencies (e.g., Prism's dependency on Aegis proto).

### 10. Verify

- `go build ./...` from root (workspace mode)
- `make swagger` in each service
- `make proto` in aegis
- `make build` in each service

## What Stays Untouched

- All business logic, domain models, handlers, services, repositories
- All test files (except import paths)
- Database migrations
- Config files
- Deploy scripts
- OPA policies

## Risk Assessment

**Low risk.** The migration is a mechanical rename — no behavioral changes. The main risk is missing an import path, caught immediately by compilation errors. The find-and-replace is comprehensive and covers all `.go` files, `.proto` files, and `go.mod` files.

## Post-Consolidation

The monorepo enables:
- Single `docker-compose.yml` at the root
- Cross-service integration tests
- Unified CI/CD pipeline
- Single `git clone` for the entire platform
