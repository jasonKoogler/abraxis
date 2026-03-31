# Monorepo Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate Aegis, Prism, and Authz into a single monorepo called Abraxis with updated module paths.

**Architecture:** Create new repo, copy files from all three repos, find-and-replace module paths in all Go/proto files, regenerate proto and swagger, verify compilation.

**Tech Stack:** Go multi-module workspace, protobuf, swaggo

**Spec:** `docs/superpowers/specs/2026-03-30-monorepo-consolidation-design.md`

---

### Task 1: Create repo structure and copy files

**Files:**
- Create: `/home/jason/jdk/abraxis/` (new repo root)
- Create: `/home/jason/jdk/abraxis/aegis/` (copy from `/home/jason/jdk/aegis/`)
- Create: `/home/jason/jdk/abraxis/prism/` (copy from `/home/jason/jdk/prism/`)
- Create: `/home/jason/jdk/abraxis/authz/` (copy from `/home/jason/jdk/authz/`)

- [ ] **Step 1: Create the GitHub repo**

```bash
gh repo create jasonKoogler/abraxis --public --description "Auth service + API gateway platform"
```

- [ ] **Step 2: Clone and set up directory structure**

```bash
cd /home/jason/jdk
git clone git@github.com:jasonKoogler/abraxis.git
cd abraxis
```

- [ ] **Step 3: Copy Aegis files (excluding .git, go.work, go.work.sum)**

```bash
rsync -av --exclude='.git' --exclude='go.work' --exclude='go.work.sum' --exclude='build/' --exclude='third_party/' /home/jason/jdk/aegis/ /home/jason/jdk/abraxis/aegis/
```

Note: `third_party/` contains cloned googleapis for proto gen — it will be re-downloaded by the proto script when needed. `build/` is the compiled binary output.

- [ ] **Step 4: Copy Prism files (excluding .git, go.work, go.work.sum)**

```bash
rsync -av --exclude='.git' --exclude='go.work' --exclude='go.work.sum' /home/jason/jdk/prism/ /home/jason/jdk/abraxis/prism/
```

- [ ] **Step 5: Copy Authz files (excluding .git)**

```bash
rsync -av --exclude='.git' --exclude='coverage.html' --exclude='coverage.out' /home/jason/jdk/authz/ /home/jason/jdk/abraxis/authz/
```

- [ ] **Step 6: Create root go.work**

```bash
cat > /home/jason/jdk/abraxis/go.work << 'EOF'
go 1.24.0

use (
	./aegis
	./prism
	./authz
)
EOF
```

- [ ] **Step 7: Create root .gitignore**

```bash
cat > /home/jason/jdk/abraxis/.gitignore << 'EOF'
# Build output
build/

# IDE
.idea/
.vscode/
*.swp

# Environment
.env

# OS
.DS_Store

# Go workspace sum
go.work.sum

# Third party (downloaded at build time)
third_party/
EOF
```

- [ ] **Step 8: Verify directory structure**

```bash
ls /home/jason/jdk/abraxis/aegis/go.mod /home/jason/jdk/abraxis/prism/go.mod /home/jason/jdk/abraxis/authz/go.mod
```

Expected: all three files exist.

- [ ] **Step 9: Commit raw copy**

```bash
cd /home/jason/jdk/abraxis
git add -A
git commit -m "chore: initial copy of aegis, prism, and authz into monorepo"
```

---

### Task 2: Update all module paths

**Files:**
- Modify: All `go.mod` files (3)
- Modify: All `.go` files with import paths (~160 files)
- Modify: `aegis/api/grpc/aegispb/aegis.proto`

Three find-and-replace operations across all Go files, go.mod files, and proto files. The order matters — do authz first (standalone, no cross-refs), then aegis, then prism.

- [ ] **Step 1: Update authz go.mod module path**

In `/home/jason/jdk/abraxis/authz/go.mod`, change:
```
module github.com/jasonKoogler/authz
```
To:
```
module github.com/jasonKoogler/abraxis/authz
```

- [ ] **Step 2: Find-and-replace authz imports in all Go files**

```bash
cd /home/jason/jdk/abraxis
find . -name "*.go" -exec sed -i 's|github.com/jasonKoogler/authz|github.com/jasonKoogler/abraxis/authz|g' {} +
```

This covers authz's own internal references (9 files), aegis's reference (1 file), and prism's reference (1 file).

- [ ] **Step 3: Update aegis go.mod module path and authz dep**

In `/home/jason/jdk/abraxis/aegis/go.mod`, change:
```
module github.com/jasonKoogler/aegis
```
To:
```
module github.com/jasonKoogler/abraxis/aegis
```

The authz dependency line was already updated by Step 2's sed on go.mod files, but verify:
```bash
grep "jasonKoogler" /home/jason/jdk/abraxis/aegis/go.mod
```

Expected: `module github.com/jasonKoogler/abraxis/aegis` and `github.com/jasonKoogler/abraxis/authz v0.3.0`

- [ ] **Step 4: Find-and-replace aegis imports in all Go files**

```bash
cd /home/jason/jdk/abraxis
find . -name "*.go" -exec sed -i 's|github.com/jasonKoogler/aegis|github.com/jasonKoogler/abraxis/aegis|g' {} +
```

This covers aegis's internal references (67 files) and prism's references to aegis proto (3 files).

- [ ] **Step 5: Update prism go.mod module path**

In `/home/jason/jdk/abraxis/prism/go.mod`, change:
```
module github.com/jasonKoogler/prism
```
To:
```
module github.com/jasonKoogler/abraxis/prism
```

- [ ] **Step 6: Find-and-replace prism imports in all Go files**

```bash
cd /home/jason/jdk/abraxis
find . -name "*.go" -exec sed -i 's|github.com/jasonKoogler/prism|github.com/jasonKoogler/abraxis/prism|g' {} +
```

This covers prism's internal references (82 files).

- [ ] **Step 7: Update proto file go_package option**

In `/home/jason/jdk/abraxis/aegis/api/grpc/aegispb/aegis.proto`, change:
```
option go_package = "github.com/jasonKoogler/aegis/api/grpc/aegispb";
```
To:
```
option go_package = "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb";
```

- [ ] **Step 8: Verify no old module paths remain**

```bash
cd /home/jason/jdk/abraxis
grep -r "jasonKoogler/aegis[^/]" --include="*.go" --include="*.proto" --include="go.mod" . | grep -v "abraxis" || echo "Clean — no old paths found"
grep -r "jasonKoogler/prism[^/]" --include="*.go" --include="go.mod" . | grep -v "abraxis" || echo "Clean — no old paths found"
grep -r "jasonKoogler/authz[^/]" --include="*.go" --include="go.mod" . | grep -v "abraxis" || echo "Clean — no old paths found"
```

Expected: all three return "Clean". If any old paths remain, fix them.

Note: The pattern `[^/]` after the module name ensures we don't false-match the already-updated `abraxis/aegis` paths. For `aegis`, we need to be careful: `jasonKoogler/aegis` should NOT match `jasonKoogler/abraxis/aegis`. The regex `jasonKoogler/aegis[^/]` catches `jasonKoogler/aegis/` (old) but not `jasonKoogler/abraxis/aegis/` (new). Wait — `jasonKoogler/aegis/` has `/` after `aegis`, and `[^/]` means "not /"... This won't catch `jasonKoogler/aegis/` because `/` is excluded by `[^/]`. Better approach:

```bash
cd /home/jason/jdk/abraxis
grep -rn "\"github.com/jasonKoogler/aegis" --include="*.go" --include="go.mod" . | grep -v "abraxis" || echo "Clean"
grep -rn "\"github.com/jasonKoogler/prism" --include="*.go" --include="go.mod" . | grep -v "abraxis" || echo "Clean"
grep -rn "\"github.com/jasonKoogler/authz" --include="*.go" --include="go.mod" . | grep -v "abraxis" || echo "Clean"
grep -rn "jasonKoogler/aegis" --include="*.proto" . | grep -v "abraxis" || echo "Clean"
```

- [ ] **Step 9: Commit module path updates**

```bash
cd /home/jason/jdk/abraxis
git add -A
git commit -m "refactor: update all module paths to github.com/jasonKoogler/abraxis/*"
```

---

### Task 3: Regenerate proto and swagger, verify compilation

**Files:**
- Modify: `aegis/api/grpc/aegispb/*.pb.go` (regenerated)
- Modify: `aegis/docs/` (regenerated swagger)
- Modify: `prism/docs/` (regenerated swagger)
- Modify: `aegis/go.mod`, `aegis/go.sum` (go mod tidy)
- Modify: `prism/go.mod`, `prism/go.sum` (go mod tidy)
- Modify: `authz/go.mod`, `authz/go.sum` (go mod tidy)

- [ ] **Step 1: Regenerate proto**

```bash
cd /home/jason/jdk/abraxis/aegis
make proto
```

This runs `scripts/protogen.sh` which uses relative paths from project root. The generated `*.pb.go` files will contain the updated `abraxis/aegis` module path from the proto's `go_package` option.

If `make proto` fails because `protoc` or plugins aren't installed, install them:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

- [ ] **Step 2: Run go mod tidy for each module**

```bash
cd /home/jason/jdk/abraxis/authz && go mod tidy
cd /home/jason/jdk/abraxis/aegis && go mod tidy
cd /home/jason/jdk/abraxis/prism && go mod tidy
```

This cleans up stale go.sum entries and properly declares cross-module deps. Run from the monorepo root so `go.work` resolves local modules.

- [ ] **Step 3: Verify full compilation**

```bash
cd /home/jason/jdk/abraxis
go build ./...
```

This builds all three modules via the workspace. Fix any remaining import errors.

- [ ] **Step 4: Regenerate swagger for both services**

```bash
cd /home/jason/jdk/abraxis/aegis && make swagger
cd /home/jason/jdk/abraxis/prism && make swagger
```

- [ ] **Step 5: Verify swagger output**

```bash
cd /home/jason/jdk/abraxis/aegis && python3 -c "import json; d=json.load(open('docs/swagger.json')); print('Aegis paths:', sorted(d['paths'].keys()))"
cd /home/jason/jdk/abraxis/prism && python3 -c "import json; d=json.load(open('docs/swagger.json')); print('Prism paths:', sorted(d['paths'].keys()))"
```

- [ ] **Step 6: Verify each service builds independently**

```bash
cd /home/jason/jdk/abraxis/aegis && go build ./cmd/...
cd /home/jason/jdk/abraxis/prism && go build ./cmd/...
```

- [ ] **Step 7: Commit regenerated files**

```bash
cd /home/jason/jdk/abraxis
git add -A
git commit -m "chore: regenerate proto and swagger with updated module paths"
```

---

### Task 4: Create root Makefile and README, test GOWORK=off

**Files:**
- Create: `/home/jason/jdk/abraxis/Makefile`
- Create: `/home/jason/jdk/abraxis/README.md`
- Modify: `aegis/Makefile` (test GOWORK=off removal)
- Modify: `prism/Makefile` (test GOWORK=off removal)

- [ ] **Step 1: Create root Makefile**

Create `/home/jason/jdk/abraxis/Makefile`:

```makefile
.DEFAULT_GOAL := help

.PHONY: build-all test-all swagger-all proto clean help

build-all: ## Build both services
	$(MAKE) -C aegis build
	$(MAKE) -C prism build

test-all: ## Run tests for all modules
	$(MAKE) -C aegis test
	$(MAKE) -C prism test

swagger-all: ## Regenerate swagger docs for both services
	$(MAKE) -C aegis swagger
	$(MAKE) -C prism swagger

proto: ## Regenerate protobuf code
	$(MAKE) -C aegis proto

clean: ## Clean build artifacts
	$(MAKE) -C aegis clean
	$(MAKE) -C prism clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
```

- [ ] **Step 2: Test whether GOWORK=off is still needed for swagger**

The swagger targets currently use `GOWORK=off` to prevent cross-module annotation leakage. Test if it's still needed in the monorepo:

```bash
cd /home/jason/jdk/abraxis/prism
# Temporarily remove GOWORK=off and test
swag init -g cmd/main.go --parseDependency --parseInternal --useStructName 2>&1 | tail -5
python3 -c "import json; d=json.load(open('docs/swagger.json')); paths=sorted(d['paths'].keys()); print(paths); print('Has auth routes:', any('/auth' in p for p in paths))"
```

If "Has auth routes: True" — the leakage still occurs and GOWORK=off must stay.
If "Has auth routes: False" — GOWORK=off can be removed from both Makefiles.

- [ ] **Step 3: Update Makefiles based on GOWORK=off test**

If GOWORK=off is still needed: leave the Makefiles as-is.

If GOWORK=off can be removed: update both service Makefiles to remove it from the swagger target:
```bash
sed -i 's/GOWORK=off swag/swag/' /home/jason/jdk/abraxis/aegis/Makefile
sed -i 's/GOWORK=off swag/swag/' /home/jason/jdk/abraxis/prism/Makefile
```

Then regenerate swagger in both services to confirm clean output.

- [ ] **Step 4: Create root README.md**

Create `/home/jason/jdk/abraxis/README.md`:

```markdown
# Abraxis

Auth service + API gateway platform.

## Services

| Service | Description | Port |
|---------|-------------|------|
| **Aegis** | Authentication, JWT (Ed25519/EdDSA), OAuth, RBAC, user management | :8080 (HTTP), :9090 (gRPC) |
| **Prism** | API gateway, service routing, rate limiting, audit logging, API keys | :8080 (HTTP) |

## Shared Libraries

| Library | Description |
|---------|-------------|
| **Authz** | OPA-based authorization with caching, RBAC, and middleware |

## Quick Start

```bash
# Build both services
make build-all

# Run tests
make test-all

# Regenerate swagger docs
make swagger-all

# Regenerate protobuf
make proto
```

## Architecture

Aegis handles authentication and issues JWT tokens signed with Ed25519. Prism validates tokens via JWKS and proxies requests to backend services. Both services communicate over gRPC for auth data sync, policy sync, and permission checks. Authz provides OPA policy evaluation used by both services.

## API Documentation

After building, swagger UI is available at:
- Aegis: `http://localhost:8080/swagger/`
- Prism: `http://localhost:8080/swagger/`
```

- [ ] **Step 5: Remove stale files from service dirs**

Clean up files that belong at the root or are no longer needed:

```bash
cd /home/jason/jdk/abraxis
# Remove per-service go.work files (now at root)
rm -f aegis/go.work aegis/go.work.sum
rm -f prism/go.work prism/go.work.sum
# Remove old docs that reference pre-monorepo structure
rm -f prism/redocly.yaml
rm -f prism/login.ts
```

- [ ] **Step 6: Remove .claude directory from aegis (session-specific, not for monorepo)**

```bash
rm -rf /home/jason/jdk/abraxis/aegis/.claude
```

- [ ] **Step 7: Final full verification**

```bash
cd /home/jason/jdk/abraxis
go build ./...
make swagger-all
make build-all
```

- [ ] **Step 8: Commit and push**

```bash
cd /home/jason/jdk/abraxis
git add -A
git commit -m "feat: add root Makefile, README, clean up stale files"
git push -u origin main
```
