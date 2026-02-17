.PHONY: fmt vet lint test vuln tidy build all docker-up docker-down migrate-up migrate-down

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test -v -race -coverprofile=coverage.out ./...

vuln:
	govulncheck ./...

tidy:
	go mod tidy
	go mod verify

build:
	CGO_ENABLED=0 go build ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	@echo "Applying migrations to $${AIRA_DB_NAME:-aira_dev}..."
	@for f in migrations/*.sql; do \
		echo "  -> $$f"; \
		PGPASSWORD=$${AIRA_DB_PASSWORD:-aira} psql \
			-h $${AIRA_DB_HOST:-localhost} \
			-p $${AIRA_DB_PORT:-5432} \
			-U $${AIRA_DB_USER:-aira} \
			-d $${AIRA_DB_NAME:-aira_dev} \
			-f "$$f" || exit 1; \
	done

migrate-down:
	@if [ "$${AIRA_ENV}" != "dev" ] && [ "$${AIRA_ENV}" != "test" ]; then \
		echo "ERROR: migrate-down requires AIRA_ENV=dev or AIRA_ENV=test (current: $${AIRA_ENV:-unset})"; \
		exit 1; \
	fi
	@echo "Dropping all tables from $${AIRA_DB_NAME:-aira_dev}..."
	PGPASSWORD=$${AIRA_DB_PASSWORD:-aira} psql \
		-h $${AIRA_DB_HOST:-localhost} \
		-p $${AIRA_DB_PORT:-5432} \
		-U $${AIRA_DB_USER:-aira} \
		-d $${AIRA_DB_NAME:-aira_dev} \
		-c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

all: fmt vet lint test vuln build
