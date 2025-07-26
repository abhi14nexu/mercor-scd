.PHONY: dev-up migrate seed test bench reset demo-build demo-help api

dev-up:
	docker compose up -d db
	@echo "⏳ Waiting for PostgreSQL to be ready..."
	sleep 3
	make migrate
	make seed
	@echo "🚀 Starting API server..."
	export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable" && go run ./cmd/api

# Reset database to clean state
reset:
	@echo "🔄 Resetting database to clean state..."
	docker compose down -v
	make dev-up
	@echo "⏳ Waiting for PostgreSQL to be ready..."
	sleep 3
	make migrate
	@echo "✅ Database reset complete!"

migrate:
	go run ./cmd/migrate up

# Build demo CLI
demo-build:
	@echo "🔨 Building demo CLI..."
	go build -o demo-cli cmd/demo/*.go
	@echo "✅ Demo CLI built successfully!"

# Seed database with full data (including version updates)
seed:
	@echo "🌱 Seeding database with full sample data..."
	export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable" && go run ./cmd/demo seed

# Demo workflow - reset, seed, and show usage examples
demo: seed demo-build
	@echo ""
	@echo "🎉 Demo database ready! Try these commands:"
	@echo ""
	@echo "📋 Query latest jobs by company:"
	@echo "   export DATABASE_URL=\"postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable\""
	@echo "   ./demo-cli latest-jobs --company=company-acme"
	@echo "   ./demo-cli latest-jobs --company=company-tech"
	@echo ""
	@echo "💰 Query payments by contractor:"
	@echo "   ./demo-cli payments --contractor=contractor-alice"
	@echo "   ./demo-cli payments --contractor=contractor-bob"
	@echo ""
	@echo "❓ Show all available commands:"
	@echo "   ./demo-cli --help"

# Show demo help
demo-help:
	@echo "🎯 SCD Demo Commands:"
	@echo ""
	@echo "  make demo        - Full demo setup (reset + seed + instructions)"
	@echo "  make seed        - Reset DB and seed with full SCD demo data"
	@echo "  make reset       - Reset database to clean state"
	@echo "  make demo-build  - Build the demo CLI binary"
	@echo ""
	@echo "🐳 Database Commands:"
	@echo "  make dev-up      - Start PostgreSQL with Docker"
	@echo "  make migrate     - Run database migrations"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test        - Run all tests"
	@echo "  make bench       - Run benchmarks"

test:
	go test ./...

bench:
	go test -bench=. ./...

# Start just the API server (assumes database is already running)
api:
	@echo "🚀 Starting API server on :8081..."
	export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable" && go run ./cmd/api 