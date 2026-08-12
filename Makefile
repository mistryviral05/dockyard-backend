APP=dockyard

# --- Database / migrations ---
# Override per environment: make migrate-up DB_URL="postgres://..."
# On a MySQL move: point MIGRATIONS_DIR at db/migrations/mysql and swap DB_URL —
# no other changes needed.
DB_URL?=postgres://postgres:password@localhost:5432/dockyard?sslmode=disable
MIGRATIONS_DIR=db/migrations/postgres

build:
	@go build -o build/$(APP) ./cmd/$(APP)/main.go

start:
	@./build/$(APP) --config ./config/local.yaml

run:
	@go run ./cmd/$(APP)/main.go --config ./config/local.yaml

run-worker:
	@go run ./cmd/worker/main.go --config ./config/local.yaml

# this is only for temporary testing of ping endpoint, not for production use
run-ping:
	@go run ./cmd/ping/main.go --config ./config/local.yaml

# Create a new empty migration pair: make migrate-create name=add_something
migrate-create:
	@migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

# Apply all pending up migrations
migrate-up:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

# Roll back the most recent migration
migrate-down:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

# Roll back everything
migrate-down-all:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down -all

# Show current migration version
migrate-version:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

# Recover from a dirty state: make migrate-force version=N
migrate-force:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" force $(version)

.PHONY: run migrate-create migrate-up migrate-down migrate-down-all migrate-version migrate-force run-worker run-ping build start
