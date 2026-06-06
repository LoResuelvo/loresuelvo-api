.PHONY: up down clean build bash lint test openapi swagger swagger-down spec gherkin test-all-once migrate-up migrate-down migrate-test-up migrate-test-down storage storage-console storage-reset

# Nombre del servicio del compose
SERVICE = api-dev

up:
	docker compose up -d $(SERVICE)

storage:
	docker compose up -d minio minio-init

storage-console:
	docker compose up -d minio minio-init
	@echo "MinIO API:     http://minio.localhost:$${MINIO_API_PORT:-9000}"
	@echo "MinIO Console: http://minio.localhost:$${MINIO_CONSOLE_PORT:-9001}"
	@echo "Credentials:   loresuelvo-local / loresuelvo-local-secret"

storage-reset:
	docker compose rm -sfv minio minio-init
	docker volume rm loresuelvo-api_minio-data || true
	docker compose up -d minio minio-init

down:
	docker compose down

# Para limpieza profunda
clean:
	docker compose down --rmi all --volumes --remove-orphans

# Fuerza la reconstrucción de la imagen (útil si se agregan dependencias en el go.mod)
build:
	docker compose build $(SERVICE)

# Para entrar en la consola del contenedor
bash:
	docker compose exec $(SERVICE) sh

# Ejecuta linter
lint:
	docker compose exec -e GOFLAGS="-buildvcs=false" $(SERVICE) golangci-lint run

test:
	docker compose exec -e GOFLAGS="-buildvcs=false" $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -p 1 -v ./...'

test-all-once:
	docker compose exec -e GOFLAGS="-buildvcs=false" $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -count=1 -p 1 -v ./... && golangci-lint run'

openapi:
	python3 scripts/bundle_openapi.py

swagger: openapi
	docker compose up -d --force-recreate $(SERVICE)
	docker compose up swagger-ui

swagger-down:
	docker compose stop swagger-ui

migrate-up:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$DATABASE_URL" up'

migrate-down:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$DATABASE_URL" down 1'

migrate-test-up:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up'

migrate-test-down:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" down 1'
