APP=dockyard

# Database migrations.
#
# These run through ./cmd/migrate, which embeds the golang-migrate library, so
# there is no CLI to install. The connection details come from the same config
# the server uses, so there is no second copy of the credentials either.
#
# Point at another environment with CONFIG:
#   make migrate-up CONFIG=./config/production.env
# Leave CONFIG empty to read everything from environment variables:
#   make migrate-up CONFIG=
CONFIG?=./config/development.env
MIGRATIONS_DIR?=db/migrations/postgres
MIGRATE=go run ./cmd/migrate --path $(MIGRATIONS_DIR) $(if $(CONFIG),--config $(CONFIG),)

build:
	@go build -o bin/$(APP) ./cmd/$(APP)/main.go

start:
	@./build/$(APP) --config ./config/development.env

run:
	@go run ./cmd/$(APP)/main.go --config ./config/development.env

run-worker:
	@go run ./cmd/worker/main.go --config ./config/development.env

# this is only for temporary testing of ping endpoint, not for production use
run-ping:
	@go run ./cmd/ping/main.go --config ./config/development.env

# Write a new empty migration pair: make migrate-create name=add_something
migrate-create:
	@$(MIGRATE) create $(name)

# Apply every pending migration
migrate-up:
	@$(MIGRATE) up

# Roll back the most recent migration, or N of them: make migrate-down n=3
migrate-down:
	@$(MIGRATE) down $(or $(n),1)

# Roll back everything
migrate-down-all:
	@$(MIGRATE) down all

# Move straight to a version: make migrate-goto version=5
migrate-goto:
	@$(MIGRATE) goto $(version)

# Show the version the database is on
migrate-version:
	@$(MIGRATE) version

# Recover from a dirty state: make migrate-force version=N
migrate-force:
	@$(MIGRATE) force $(version)

.PHONY: run migrate-create migrate-up migrate-down migrate-down-all migrate-goto migrate-version migrate-force run-worker run-ping build start
