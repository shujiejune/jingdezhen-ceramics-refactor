# Use the development compose file by default for all commands.
COMPOSE_FILE = -f docker-compose.dev.yml

# Load environment variables from .env file for use in this Makefile
# This makes sure DATABASE_URL is available for the migrate commands.
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: help up down stop logs migrate-up migrate-down db-seed

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          - Build the development Docker images"
	@echo "  up             - Start all services in detached mode"
	@echo "  down           - Stop and remove all services and volumes"
	@echo "  stop           - Stop all running services"
	@echo "  logs           - View logs for all services"
	@echo "  migrate-up     - Apply all new database migrations"
	@echo "  migrate-down   - Roll back the last database migration"
	@echo "  db-seed        - Seed the database with initial test data"

build:
	@echo "Building Docker images..."
	docker compose $(COMPOSE_FILE) build

up:
	@echo "Starting Docker containers..."
	docker compose $(COMPOSE_FILE) up -d

down:
	@echo "Stopping and removing Docker containers and volumes..."
	docker compose $(COMPOSE_FILE) down -v

stop:
	@echo "Stopping Docker containers..."
	docker compose $(COMPOSE_FILE) stop

logs:
	@echo "Tailing logs..."
	docker compose $(COMPOSE_FILE) logs -f

migrate-up:
	@echo "Applying database migrations..."
	migrate -database "${DATABASE_URL}" -path internal/migrations up

migrate-down:
	@echo "Rolling back last database migration..."
	migrate -database "${DATABASE_URL}" -path internal/migrations down

db-seed:
	@echo "Seeding database with test data..."
	psql "${DATABASE_URL}" -f internal/migrations/seed.sql
