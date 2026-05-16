.PHONY: up down clean build bash lint test spec gherkin test-all-once migrate-up migrate-down migrate-test-up migrate-test-down

# Nombre del servicio del compose
SERVICE = api-dev

up:
	docker compose up -d $(SERVICE)

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
	docker compose exec -e GOFLAGS="-buildvcs=false" $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -v ./...'

test-all-once:
	docker compose exec -e GOFLAGS="-buildvcs=false" $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -v ./... && golangci-lint run'

migrate-up:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$DATABASE_URL" up'

migrate-down:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$DATABASE_URL" down 1'

migrate-test-up:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" up'

migrate-test-down:
	docker compose exec $(SERVICE) sh -c 'migrate -path db/migrations -database "$$TEST_DATABASE_URL" down 1'
