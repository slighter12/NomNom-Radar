.PHONY: help test-race test-usecase-race lint \
    sec-scan trivy-scan vuln-scan \
    db-postgres-init db-postgres-seeders-init \
    db-postgres-create db-postgres-up db-postgres-down db-postgres-down-all \
    db-postgres-status db-postgres-install-goose \
	gci-format build docker-image-build \
	docker-up docker-down docker-logs docker-clean \
	k6-full \
	routing-cli routing-prepare routing-validate \
	generate-mocks

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

PROJECT_NAME ?= radar
SQL_FILE_TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
GitCommit := $(shell git rev-parse HEAD)
Date := $(shell date -Iseconds)
SHELL := /bin/bash
DOCKER_PLATFORM ?= linux/amd64
TEST_PKGS ?= ./...
USECASE_TEST_PKGS ?= ./internal/usecase/impl/...
K6 ?= $(shell if command -v k6 >/dev/null 2>&1; then command -v k6; elif command -v mise >/dev/null 2>&1 && mise which k6 >/dev/null 2>&1; then mise which k6; else echo k6; fi)
K6_BASE_URL ?= http://localhost:4433
K6_RUN_ID ?= $(shell date '+%Y%m%d%H%M%S')
K6_TEST_PASSWORD ?= K6pass!1234
FULL_VUS ?= 1
FULL_ITERATIONS ?= 1
FULL_MAX_DURATION ?= 5m
FULL_SLEEP_SECONDS ?= 0

########
# test #
########

test-race: ## launch tests with race detection (override with TEST_PKGS=./path/...)
	go test -p 4 $(TEST_PKGS) -cover -race

test-usecase-race: TEST_PKGS=$(USECASE_TEST_PKGS)
test-usecase-race: ## launch usecase tests with race detection
	go test -p 4 $(TEST_PKGS) -cover -race

########
# lint #
########

lint: ## lints the entire codebase
	@golangci-lint run ./... --config=./.golangci.yaml

#######
# sec #
#######

sec-scan: trivy-scan vuln-scan ## scan for security and vulnerability issues

trivy-scan: ## scan for sec issues with trivy (trivy binary needed)
	trivy fs --exit-code 1 --no-progress --severity CRITICAL ./

vuln-scan: ## scan for vulnerability issues with govulncheck (govulncheck binary needed)
	govulncheck ./...

######
# db #
######
POSTGRES_SQL_PATH := ./database/migration/postgres
POSTGRES_SEEDERS_SQL_PATH := ./database/migration/postgres/seeders
POSTGRES_HOST ?= localhost
POSTGRES_PORT ?=
POSTGRES_DB_NAME ?= auth_db
POSTGRES_DB_USER ?= user
POSTGRES_DB_PASSWORD ?= password
POSTGRES_SSLMODE ?= disable
PG_URI ?=
GOOSE ?= $(shell if command -v goose >/dev/null 2>&1; then command -v goose; elif [ -x "$$(go env GOPATH)/bin/goose" ]; then echo "$$(go env GOPATH)/bin/goose"; else echo goose; fi)

# -----------------------------------------------------------------------------
# PostgreSQL
# -----------------------------------------------------------------------------

db-postgres-init: ## initialize new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	$(GOOSE) -dir ${POSTGRES_SQL_PATH} create init sql

db-postgres-seeders-init: ## initialize new PostgreSQL seeder
	@mkdir -p ${POSTGRES_SEEDERS_SQL_PATH}
	@( \
		SEEDER_NAME="$(NAME)"; \
		if [ -z "$${SEEDER_NAME}" ]; then printf "Enter seeder name: "; read -r SEEDER_NAME; fi; \
		if [ -z "$${SEEDER_NAME}" ]; then echo "NAME is required"; exit 1; fi; \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.up.sql && \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.down.sql \
	)

db-postgres-create: ## create new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	@( \
		MIGRATE_NAME="$(NAME)"; \
		if [ -z "$${MIGRATE_NAME}" ]; then printf "Enter migration name: "; read -r MIGRATE_NAME; fi; \
		if [ -z "$${MIGRATE_NAME}" ]; then echo "NAME is required"; exit 1; fi; \
		$(GOOSE) -dir ${POSTGRES_SQL_PATH} create "$${MIGRATE_NAME}" sql \
	)

define postgres_uri_command
	PG_URI_VALUE="$(PG_URI)"; \
	if [ -z "$${PG_URI_VALUE}" ]; then \
		PG_PORT_VALUE="$(POSTGRES_PORT)"; \
		if [ -z "$${PG_PORT_VALUE}" ]; then printf "Enter port(5432...): "; read -r PG_PORT_VALUE; fi; \
		PG_PORT_VALUE=$${PG_PORT_VALUE:-5432}; \
		PG_URI_VALUE="postgres://${POSTGRES_DB_USER}:${POSTGRES_DB_PASSWORD}@${POSTGRES_HOST}:$${PG_PORT_VALUE}/${POSTGRES_DB_NAME}?sslmode=${POSTGRES_SSLMODE}"; \
	fi
endef

db-postgres-up: ## apply all PostgreSQL migrations
	@( \
		$(postgres_uri_command); \
		$(GOOSE) -dir ${POSTGRES_SQL_PATH} postgres "$${PG_URI_VALUE}" up \
	)

db-postgres-down: ## revert all PostgreSQL migrations
	@( \
		$(postgres_uri_command); \
		$(GOOSE) -dir ${POSTGRES_SQL_PATH} postgres "$${PG_URI_VALUE}" down \
	)

db-postgres-down-all: ## revert PostgreSQL migrations down to version 0 (drops all applied migrations)
	@( \
		$(postgres_uri_command); \
		$(GOOSE) -dir ${POSTGRES_SQL_PATH} postgres "$${PG_URI_VALUE}" down-to 0 \
	)

db-postgres-status: ## show PostgreSQL migration status
	@( \
		$(postgres_uri_command); \
		$(GOOSE) -dir ${POSTGRES_SQL_PATH} postgres "$${PG_URI_VALUE}" status \
	)

db-postgres-install-goose: ## install goose CLI
	go install github.com/pressly/goose/v3/cmd/goose@latest

db-postgres-test-replication: ## test replication
	@echo "Testing replication with SCRAM-SHA-256 authentication..."
	@echo "Creating test table on master..."
	docker exec -e PGPASSWORD=password radar-postgres-master psql -U user -d auth_db -c "CREATE TABLE IF NOT EXISTS test_replication (id SERIAL PRIMARY KEY, message TEXT, created_at TIMESTAMP DEFAULT NOW());"
	@echo "Inserting test data on master..."
	docker exec -e PGPASSWORD=password radar-postgres-master psql -U user -d auth_db -c "INSERT INTO test_replication (message) VALUES ('Test from master at $$(date)');"
	@echo "Checking data on replica..."
	docker exec -e PGPASSWORD=password radar-postgres-replica psql -U user -d auth_db -c "SELECT * FROM test_replication ORDER BY id DESC LIMIT 1;"

###########
#   GCI   #
###########

GCI_DOMAIN_PREFIX ?=

gci-format: ## format imports
	gci write --skip-generated -s standard -s default $$(if $(GCI_DOMAIN_PREFIX),-s "prefix($${GCI_DOMAIN_PREFIX})",) -s "prefix($(PROJECT_NAME))" ./

#########
# build #
#########

build: ## build the project
	@( \
		printf "Enter version: "; read -r VERSION; \
		go build -ldflags "-s -w -X 'main.Version=$$VERSION' -X 'main.Built=$(Date)' -X 'main.GitCommit=$(GitCommit)'" -o ./bin/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME) \
	)

docker-image-build: ## build Docker image
	docker build \
		-t $(PROJECT_NAME) \
		--platform $(DOCKER_PLATFORM) \
		--build-arg BUILT=$(Date) \
		--build-arg GIT_COMMIT=$(GitCommit) \
		./

###########
# Docker  #
###########

docker-up: ## start PostgreSQL services with Docker Compose
	docker-compose up -d

docker-down: ## stop PostgreSQL services with Docker Compose
	docker-compose down

docker-restart: ## restart PostgreSQL services with Docker Compose
	$(MAKE) docker-down
	$(MAKE) docker-up

docker-logs: ## show logs from PostgreSQL services
	docker-compose logs -f

docker-clean: ## remove all containers, networks, and volumes
	docker-compose down -v --remove-orphans

k6-full: ## run k6 functional API verification with local k6
	BASE_URL="$(K6_BASE_URL)" \
	RUN_ID="$(K6_RUN_ID)" \
	K6_TEST_PASSWORD="$(K6_TEST_PASSWORD)" \
	FULL_VUS="$(FULL_VUS)" \
	FULL_ITERATIONS="$(FULL_ITERATIONS)" \
	FULL_MAX_DURATION="$(FULL_MAX_DURATION)" \
	FULL_SLEEP_SECONDS="$(FULL_SLEEP_SECONDS)" \
	$(K6) run k6/full.js

#############
#  Routing  #
#############

routing-cli: ## build the routing CLI tool
	go build -o bin/routing-cli ./cmd/routing

routing-prepare: routing-cli ## prepare routing data for Taiwan
	./bin/routing-cli prepare --region taiwan --output ./data/routing

routing-validate: routing-cli ## validate routing data
	./bin/routing-cli validate --dir ./data/routing

generate-mocks: ## generate mocks for all interfaces
	go generate ./...
