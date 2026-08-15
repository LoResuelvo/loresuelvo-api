.PHONY: up dev-proxy down clean build bash lint lint-ci test test-ci openapi swagger swagger-down spec gherkin test-all-once migrate-up migrate-down migrate-test-up migrate-test-down storage storage-console storage-reset seed-assets-local generate-provider-seeds

# Nombre del servicio del compose
SERVICE = api-dev

up:
	docker compose up -d $(SERVICE)

dev-proxy:
	docker compose up -d nginx-dev
	@echo "Development gateway: $${DEV_PUBLIC_URL:-http://localhost:$${NGINX_DEV_PORT:-8082}}"

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

seed-assets-local:
	docker compose up -d minio minio-init
	docker compose run --rm --entrypoint /bin/sh minio-init /minio-init/upload-seed-assets.sh

generate-provider-seeds:
	python3 scripts/generate_provider_seed.py \
		--count 100 \
		--output seeds/providers-100.yaml \
		--manifest seeds/providers-100-assets.tsv \
		--assets-dir seeds/assets/provider_profile_photo

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

# CI uses one-off containers so the API and development database do not need to
# remain running while tests and lint execute.
lint-ci:
	docker compose run --rm --no-deps \
		-e GOFLAGS="-buildvcs=false" \
		$(SERVICE) golangci-lint run

test:
	@docker compose run --rm minio-clean-tests >/dev/null 2>&1
	docker compose exec \
		-e GOFLAGS="-buildvcs=false" \
		$(SERVICE) sh -c 'export STORAGE_PUBLIC_BUCKET="$$TEST_STORAGE_PUBLIC_BUCKET"; export STORAGE_PRIVATE_BUCKET="$$TEST_STORAGE_PRIVATE_BUCKET"; export STORAGE_PUBLIC_BASE_URL="$$TEST_STORAGE_PUBLIC_BASE_URL"; migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -p 1 -v ./...'; status=$$?; docker compose run --rm minio-clean-tests >/dev/null 2>&1; exit $$status

test-ci:
	@docker compose run --rm minio-clean-tests >/dev/null 2>&1
	docker compose run --rm --no-deps \
		-e GOFLAGS="-buildvcs=false" \
		$(SERVICE) sh -c 'export STORAGE_PUBLIC_BUCKET="$$TEST_STORAGE_PUBLIC_BUCKET"; export STORAGE_PRIVATE_BUCKET="$$TEST_STORAGE_PRIVATE_BUCKET"; export STORAGE_PUBLIC_BASE_URL="$$TEST_STORAGE_PUBLIC_BASE_URL"; migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -count=1 -p 1 -v ./...'; status=$$?; docker compose run --rm minio-clean-tests >/dev/null 2>&1; exit $$status

test-all-once:
	@docker compose run --rm minio-clean-tests >/dev/null 2>&1
	docker compose exec \
		-e GOFLAGS="-buildvcs=false" \
		$(SERVICE) sh -c 'export STORAGE_PUBLIC_BUCKET="$$TEST_STORAGE_PUBLIC_BUCKET"; export STORAGE_PRIVATE_BUCKET="$$TEST_STORAGE_PRIVATE_BUCKET"; export STORAGE_PUBLIC_BASE_URL="$$TEST_STORAGE_PUBLIC_BASE_URL"; migrate -path db/migrations -database "$$TEST_DATABASE_URL" up && go test -count=1 -p 1 -v ./... && golangci-lint run'; status=$$?; docker compose run --rm minio-clean-tests >/dev/null 2>&1; exit $$status

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
