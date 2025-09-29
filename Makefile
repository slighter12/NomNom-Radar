.PHONY: help test-race lint \
	sec-scan trivy-scan vuln-scan \
	db-postgres-init db-postgres-seeders-init \
	db-postgres-create db-postgres-up db-postgres-down \
	gci-format build docker-image-build \
	docker-up docker-down docker-logs docker-clean

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

PROJECT_NAME ?= radar
SQL_FILE_TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
GitCommit := $(shell git rev-parse HEAD)
Date := $(shell date -Iseconds)
SHELL := /bin/bash
DOCKER_PLATFORM ?= linux/amd64

########
# test #
########

test-race: ## launch all tests with race detection
	go test -p 4 ./... -cover -race

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
POSTGRES_DB_NAME := auth_db
POSTGRES_DB_USER := user
POSTGRES_DB_PASSWORD := password

# -----------------------------------------------------------------------------
# PostgreSQL
# -----------------------------------------------------------------------------

db-postgres-init: ## initialize new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	goose create init sql -dir ${POSTGRES_SQL_PATH}

db-postgres-seeders-init: ## initialize new PostgreSQL seeder
	@mkdir -p ${POSTGRES_SEEDERS_SQL_PATH}
	@( \
		printf "Enter seeder name: "; read -r SEEDER_NAME && \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.up.sql && \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.down.sql \
	)

db-postgres-create: ## create new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	@( \
		printf "Enter migration name: "; read -r MIGRATE_NAME && \
		goose create $${MIGRATE_NAME} sql -dir ${POSTGRES_SQL_PATH} \
	)

define goose_migrate_command
	PG_URI="postgres://${POSTGRES_DB_USER}:${POSTGRES_DB_PASSWORD}@localhost:${PG_PORT}/${POSTGRES_DB_NAME}?sslmode=disable"
	goose postgres -dir $(1) $(2) $${PG_URI}
endef

db-postgres-up: ## apply all PostgreSQL migrations
	@( \
		printf "Enter port(5432...): "; read -r PG_PORT && \
		PG_PORT=$${PG_PORT:-5432} && \
		PG_URI="postgres://${POSTGRES_DB_USER}:${POSTGRES_DB_PASSWORD}@localhost:${PG_PORT}/${POSTGRES_DB_NAME}?sslmode=disable" && \
		goose postgres "$${PG_URI}" -dir ${POSTGRES_SQL_PATH} up \
	)

db-postgres-down: ## revert all PostgreSQL migrations
	@( \
		printf "Enter port(5432...): "; read -r PG_PORT && \
		PG_PORT=$${PG_PORT:-5432} && \
		PG_URI="postgres://${POSTGRES_DB_USER}:${POSTGRES_DB_PASSWORD}@localhost:${PG_PORT}/${POSTGRES_DB_NAME}?sslmode=disable" && \
		goose postgres "$${PG_URI}" -dir ${POSTGRES_SQL_PATH} down \
	)

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
