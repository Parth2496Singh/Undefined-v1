.PHONY: db-up db-down db-psql migrate-up run-all test

db-up:
	docker compose up -d

db-down:
	docker compose down velzion-db -v

db-psql:
	docker exec -it velzion-db psql -U velzion_user -d velzion_dev

run-all: dev

run-frontend:
	cd frontend && npm run dev

dev: db-up
	@echo "Booting all Go Microservices and Frontend concurrently..."
	@export $$(grep -v '^#' .env | xargs); \
	(cd services/auth && go run cmd/api/main.go) & \
	(cd services/velzard && go run cmd/api/main.go) & \
	(cd services/zegion && go run cmd/api/main.go) & \
	(cd services/telemetry && go run cmd/api/main.go) & \
	(cd frontend && npm run dev) & \
	wait

test:
	@echo "Testing all services..."
	go test ./...

migrate-up:
	docker exec -i velzion-db psql -U velzion_user -d velzion_dev < services/auth/migrations/001_create_users_table.sql
	docker exec -i velzion-db psql -U velzion_user -d velzion_dev < services/auth/migrations/002_add_is_admin.sql
	docker exec -i velzion-db psql -U velzion_user -d velzion_dev < services/velzard/migrations/002_create_deployments_table.sql
	docker exec -i velzion-db psql -U velzion_user -d velzion_dev < services/zegion/migrations/003_create_ephemeral_envs_table.sql
