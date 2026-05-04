.PHONY: up down build bash lint test gherkin test-all-once

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
	docker compose exec $(SERVICE) golangci-lint run

test:
	docker compose exec $(SERVICE) go test -v ./...

aceptance:
	docker compose exec $(SERVICE) godog

# Levanta un contenedor temporal, corre linter, pruebas unitarias y gherkin.
# Al finalizar, el contenedor se elimina automáticamente (--rm).
test-all-once:
	docker compose run --rm $(SERVICE) sh -c "golangci-lint run && go test -v ./... && godog"