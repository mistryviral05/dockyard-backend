APP=dockyard
AGENT_VERSION?=dev

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

# The agent reads a different file: it runs on the user's server and has no
# database, Clerk or GitHub credentials at all.
AGENT_CONFIG?=./config/development.agent.env

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

run-agent:
	@go run ./cmd/agent/main.go --config $(AGENT_CONFIG)

# The agent ships as its own static binary: users download it, we do not run it.
build-agent:
	@go build -ldflags "-X github.com/mistryviral05/dockyard/internal/agentd.Version=$(AGENT_VERSION)" -o bin/dockyard-agent ./cmd/agent

# this is only for temporary testing of ping endpoint, not for production use
run-ping:
	@go run ./cmd/ping/main.go --config ./config/development.env

BENCH_HOST?=http://localhost:8040
BENCH_AGENT?=agent_zbo6rtbamqn8niqhicfqg2ui990ujmo5
BENCH_WINDOW?=15m
BENCH_CONN?=50
BENCH_DURATION?=20
BENCH_RATE?=62
SEED_COUNT?=50
AGENT_TOKEN?=

BENCH_URL="$(BENCH_HOST)/api/v1/agents/$(BENCH_AGENT)/metrics/bench?window=$(BENCH_WINDOW)"

bench:
	@autocannon -c $(BENCH_CONN) -d $(BENCH_DURATION) $(BENCH_URL)

bench-latency:
	@autocannon -c $(BENCH_CONN) -d $(BENCH_DURATION) -l $(BENCH_URL)

bench-rate:
	@autocannon -c 10 -R $(BENCH_RATE) -d $(BENCH_DURATION) $(BENCH_URL)

bench-heavy:
	@$(MAKE) bench BENCH_WINDOW=6h BENCH_CONN=100 BENCH_DURATION=30

bench-seed:
	@test -n "$(AGENT_TOKEN)" || { echo "AGENT_TOKEN is required: make bench-seed AGENT_TOKEN=agt_xxx"; exit 1; }
	@echo "sending $(SEED_COUNT) heartbeats to $(BENCH_HOST)"
	@i=1; while [ $$i -le $(SEED_COUNT) ]; do \
		curl -s -o /dev/null -w "%{http_code} " \
			-X POST "$(BENCH_HOST)/api/v1/agents/heartbeat" \
			-H "Authorization: Bearer $(AGENT_TOKEN)" \
			-H "Content-Type: application/json" \
			-d "{\"cpu_usage\":$$(( (i * 7) % 100 )),\"ram_usage\":$$(( (i * 13) % 100 )),\"disk_usage\":49.7,\"load_1\":$$(( i % 9 )),\"load_5\":2.1,\"load_15\":1.4,\"cpu_cores\":16,\"uptime_seconds\":$$(( i * 60 )),\"os_info\":\"ubuntu 24.04 (x86_64)\"}"; \
		i=$$(( i + 1 )); \
	done; echo ""

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

.PHONY: run migrate-create migrate-up migrate-down migrate-down-all migrate-goto migrate-version migrate-force run-worker run-agent build-agent run-ping build start bench bench-latency bench-rate bench-heavy bench-seed
