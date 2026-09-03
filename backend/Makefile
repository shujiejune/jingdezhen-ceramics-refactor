# Use the development compose file by default for all commands.
# Usage: `make <target>` (e.g. `make up`, `make down`, `make logs`).
COMPOSE = docker compose -f docker-compose.dev.yml

# Load environment variables from .env file for use in this Makefile
# This makes sure DATABASE_URL is available for the migrate commands.
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: help build up up-db down down-v stop logs logs-api logs-worker \
	migrate-up migrate-down migrate-force db-shell db-reset db-seed \
	test test-unit test-integration swag swag-fmt swag-check \
	k6-smoke k6-browse k6-checkout k6-spike k6-soak backup restore

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Docker lifecycle:"
	@echo "  build          - Build the development Docker images"
	@echo "  up             - Start all services (api, worker, db, redis)"
	@echo "  up-db          - Start only db + redis (for running the API on host)"
	@echo "  down           - Stop and remove containers (KEEPS data volumes)"
	@echo "  down-v         - Stop and remove containers AND data volumes (wipe)"
	@echo "  stop           - Stop containers (keep them, no removal)"
	@echo ""
	@echo "Logs:"
	@echo "  logs           - Tail logs for all services"
	@echo "  logs-api       - Tail api service logs"
	@echo "  logs-worker    - Tail worker service logs"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up     - Apply all pending migrations (needs migrate CLI)"
	@echo "  migrate-down   - Roll back the last migration"
	@echo "  migrate-force  - Force-set migration version (recover from dirty)"
	@echo "  db-shell        - Open a psql shell in the db container"
	@echo "  db-reset        - Truncate commerce tables (dev clean slate; preserves users)"
	@echo "  db-seed        - Seed dev data from scripts/seed/[0-9][0-9]_*.sql (reset + all seeds)"
	@echo "  test           - Run all tests (unit + integration via testcontainers)"
	@echo "  test-unit      - Run only unit tests (-short, no Docker needed)"
	@echo "  test-integration - Run only integration tests (needs Docker for testcontainers)"
	@echo ""
	@echo "k6 load tests (PRD §2.4.3):"
	@echo "  k6-smoke       - Post-deploy smoke gate (< 2 min)"
	@echo "  k6-browse      - Browse-heavy baseline (50 RPS, 200 VUs)"
	@echo "  k6-checkout    - Checkout funnel (10 orders/min)"
	@echo "  k6-spike       - 10× traffic spike"
	@echo "  k6-soak        - 2h soak (leak detection)"
	@echo ""
	@echo "Backup/restore (PRD §4.5):"
	@echo "  backup         - Nightly pg_dump → OSS"
	@echo "  restore        - Download + pg_restore from OSS"

build:
	@echo "Building Docker images..."
	$(COMPOSE) build

up:
	@echo "Starting Docker containers..."
	$(COMPOSE) up -d

up-db:
	@echo "Starting db + redis containers..."
	$(COMPOSE) up -d db redis

down:
	@echo "Stopping and removing containers (keeping data volumes)..."
	$(COMPOSE) down

down-v:
	@echo "Stopping and removing containers AND data volumes..."
	$(COMPOSE) down -v

stop:
	@echo "Stopping Docker containers..."
	$(COMPOSE) stop

logs:
	@echo "Tailing logs..."
	$(COMPOSE) logs -f

logs-api:
	$(COMPOSE) logs -f api

logs-worker:
	$(COMPOSE) logs -f worker

migrate-up:
	@echo "Applying database migrations..."
	migrate -database "${DATABASE_URL}" -path internal/migrations up

migrate-down:
	@echo "Rolling back last database migration..."
	migrate -database "${DATABASE_URL}" -path internal/migrations down

# Recover from a dirty migration state: `make migrate-force V=6`
migrate-force:
	@test -n "$(V)" || (echo "Usage: make migrate-force V=<version>" && exit 1)
	@echo "Forcing migration version to $(V)..."
	migrate -database "${DATABASE_URL}" -path internal/migrations force $(V)

db-shell:
	@echo "Opening psql shell in db container..."
	$(COMPOSE) exec db psql -U "$(DB_USER)" -d "$(DB_NAME)"

db-reset:
	@echo "Resetting commerce tables (dev clean slate)..."
	$(COMPOSE) exec -T db psql -U "$(DB_USER)" -d "$(DB_NAME)" < scripts/seed/00_reset.sql

## test: run all tests. Integration tests spin up real PG + Redis via
## testcontainers-go (needs Docker on the host). Set TESTCONTAINERS_RYUK_DISABLED=true
## in CI where the runner is ephemeral.
test:
	go test -race ./...

## test-unit: unit tests only (no Docker). Use this for the fast inner loop.
test-unit:
	go test -race -short ./...

## test-integration: integration tests only (needs Docker for testcontainers).
test-integration:
	go test -race -run Integration ./...

db-seed:
	@echo "Seeding database with dev test data..."
	cat scripts/seed/[0-9][0-9]_*.sql | $(COMPOSE) exec -T db psql -U "$(DB_USER)" -d "$(DB_NAME)"

## swag: regenerate the OpenAPI/Swagger 2.0 spec from handler annotations.
## Run after adding/changing @-annotations on handlers. The generated files
## in internal/docs/ ARE committed (so CI can detect drift via `swag-check`).
## --parseDependency --parseInternal let swag resolve model structs in
## internal/models (another package) from the @Success/@Param annotations.
swag:
	swag init -g cmd/api/main.go -o internal/docs --parseDependency --parseInternal

## swag-fmt: normalize the @-annotation comment formatting (align, order).
swag-fmt:
	swag fmt -d internal/modules

## swag-check: CI gate — fails if the committed internal/docs/ is stale.
## Catches "added a handler but forgot to regenerate the spec". Does NOT
## catch wrong annotations (those fail at `swag init` time with a parse error).
swag-check: swag
	@git diff --exit-code internal/docs

## k6 load tests (PRD §2.4.3). Requires k6 installed (go install go.k6.io/k6@latest).
## Set BASE_URL to target staging (default: http://localhost:1323 dev).
k6-smoke:
	k6 run k6/smoke.js

k6-browse:
	k6 run k6/browse-baseline.js

k6-checkout:
	k6 run k6/checkout-funnel.js

k6-spike:
	k6 run k6/spike.js

k6-soak:
	k6 run k6/soak.js

## backup: nightly pg_dump → OSS (PRD §4.5). Needs DATABASE_URL + OSS_* env vars.
backup:
	@bash scripts/backup/backup.sh

## restore: download + pg_restore from OSS (PRD §4.5 RTO 4h).
restore:
	@bash scripts/backup/restore.sh
