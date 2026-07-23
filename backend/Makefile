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
        migrate-up migrate-down migrate-force db-shell db-seed

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
	@echo "  db-seed        - Seed the database with dev test data (scripts/seed/seed.sql)"

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

db-seed:
	@echo "Seeding database with dev test data..."
	$(COMPOSE) exec -T db psql -U "$(DB_USER)" -d "$(DB_NAME)" < scripts/seed/seed.sql
