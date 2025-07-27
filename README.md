# SCD (Slowly Changing Dimensions) Application

A production-ready Go application demonstrating robust versioned data management with Type 2 SCD implementation, featuring clean abstractions that hide complexity, REST APIs, CLI tools, visual dashboard, efficient querying, comprehensive testing, and performance benchmarks.

## Loom Video

[![Watch the demo on Loom](https://img.shields.io/badge/Watch-Demo%20Video-ff4c42?logo=loom&logoColor=white)](https://www.loom.com/share/c4ca8f192c2e49dfb5af664c95992645?sid=1ba31f78-0bf4-4863-b87f-50a604d30b20)

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- Make

### Setup & Run

**Option 1 - Quick Demo (Recommended):**
```bash
# Set PostgreSQL environment variables
export POSTGRES_DB=mercor
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable"

# Fastest path: reset + seed + CLI build with examples
make demo
```

**Option 2 - Full Development Setup:**

**Terminal 1 - Start the Application:**
```bash
# Set PostgreSQL environment variables
export POSTGRES_DB=mercor
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mercor?sslmode=disable"

# Start PostgreSQL, run migrations, seed data, and launch API server
make dev-up
```

**Terminal 2 - Test & Explore:**
```bash
# Test CLI commands
go run cmd/demo/main.go jobs --company company-acme
go run cmd/demo/main.go history --job-id job-1
go run cmd/demo/main.go update-job --job-id job-1 --price 75.0

# Test API endpoints
curl http://localhost:8081/api/v1/health
curl http://localhost:8081/api/v1/jobs
curl http://localhost:8081/api/v1/jobs/job-1/versions
```

## Architecture Overview

### System Architecture
![SCD Architecture](https://raw.githubusercontent.com/abhi14nexu/Assets/refs/heads/main/ARCH.jpg)

### Core Components
- **SCD Library** (`internal/scd/`) - Temporal data management
- **REST API** (`cmd/api/`) - Gin server on port 8081
- **CLI Tools** (`cmd/demo/`) - Command-line interface
- **Visual Dashboard** (`ui/dashboard.html`) - Web interface
- **Database Admin** - Adminer on port 8082

## Project Structure

```
mercor-scd/
├── cmd/                          # Application entry points
│   ├── api/                      # REST API server
│   │   ├── main.go              # API server entry point
│   │   └── handlers.go          # HTTP request handlers
│   ├── demo/                     # CLI demonstration tools
│   │   ├── main.go              # CLI entry point with cobra
│   │   ├── jobs.go              # Job-related CLI commands
│   │   └── seed.go              # Database seeding logic
│   │  
│   └── migrate/                  # Database migration runner
│       └── main.go              # Migration execution
├── internal/                     # Private application code
│   ├── scd/                     # SCD library core
│   │   ├── model.go             # Base SCD model and interface
│   │   ├── update.go            # Version creation and updates
│   │   ├── scopes.go            # Query scopes and filters
│   │   └── *_test.go            # Comprehensive test suite
│   └── models/                   # Domain models
│       ├── job.go               # Job entity with SCD embedding
│       ├── timelog.go           # Timelog entity
│       └── payment_line_item.go # Payment entity
├── migrations/                   # Database schema changes
│   ├── 20250725103850_0001_init.up.sql    # Initial schema
│   └── 20250725103850_0001_init.down.sql  # Rollback schema
├── ui/                           # Frontend assets
│   └── dashboard.html           # Visual data browser
├── docker-compose.yml           # PostgreSQL and Adminer services
├── Makefile                     # Development commands and automation
├── go.mod                       # Go module dependencies
├── go.sum                       # Dependency checksums
```

### Key Features
- ✅ **Type 2 SCD** - Complete audit trails with versioning
- ✅ **Thread-Safe** - Concurrent update handling
- ✅ **Performance Optimized** - Partial indexes for latest queries
- ✅ **Clean API** - Simple abstractions over complex SCD logic

## Available Commands

### Development
```bash
make demo        # Quick demo setup (reset + seed + CLI build)
make dev-up      # Start PostgreSQL + API server
make reset       # Reset database to clean state
make test        # Run all tests
make bench       # Run benchmarks
```

### CLI Demo Commands
```bash
# Job management
go run cmd/demo/main.go jobs --company company-acme
go run cmd/demo/main.go history --job-id job-1
go run cmd/demo/main.go update-job --job-id job-1 --price 75.0

# Database operations
go run cmd/demo/main.go seed        # Seed test data
go run cmd/demo/main.go migrate     # Run migrations
```

### API Endpoints
```bash
# Health check
curl http://localhost:8081/api/v1/health

# Jobs
curl http://localhost:8081/api/v1/jobs
curl http://localhost:8081/api/v1/jobs/job-1
curl http://localhost:8081/api/v1/jobs/job-1/versions

# Payments
curl http://localhost:8081/api/v1/payments
curl http://localhost:8081/api/v1/payments/payment-1/versions

# Timelogs
curl http://localhost:8081/api/v1/timelogs
curl http://localhost:8081/api/v1/timelogs/timelog-1/versions
```

## SCD Implementation

### Data Model
```go
type Model struct {
    UID       uuid.UUID  `gorm:"primaryKey"`
    ID        string     `gorm:"index,unique:id_version"`
    Version   int        `gorm:"index,unique:id_version"`
    ValidFrom time.Time  `gorm:"not null"`
    ValidTo   *time.Time `gorm:"index"`
}
```

### Key Operations
```go
// Create new entity
job, err := scd.CreateNew[*Job](db, &Job{ID: "job-123", Rate: 50.0})

// Update creates new version
updated, err := scd.Update[*Job](db, "job-123", func(j *Job) {
    j.Rate = 60.0
})

// Query latest versions
var jobs []Job
db.Scopes(scd.Latest).Where("status = ?", "active").Find(&jobs)
```

### Query Scopes
- `scd.Latest` - Current versions only (`valid_to IS NULL`)
- `scd.Historical` - Historical versions only
- `scd.AsOf(time)` - Point-in-time queries
- `scd.ByBusinessID(id)` - All versions of entity

## Database Schema

### Core Tables
- **jobs** - Employment contracts with rates and status
- **timelogs** - Work time tracking entries
- **payment_line_items** - Financial transactions

### Performance Indexes
```sql
-- Latest version queries (90% of traffic)
CREATE INDEX idx_jobs_latest_company ON jobs(company_id) WHERE valid_to IS NULL;

-- Historical queries
CREATE INDEX idx_jobs_id ON jobs(id);
```

## Testing

### Run Tests
```bash
make test              # All tests
make test-scd          # SCD library tests only
make test-coverage     # With coverage report
make bench             # Performance benchmarks
```

### Test Data
The application includes comprehensive test data:
- 10 jobs with 3 versions each (rate changes)
- 40 timelogs linked to jobs
- Payment line items with calculated amounts

## Access Points

| Service | URL | Purpose |
|---------|-----|---------|
| API Server | http://localhost:8081 | REST API endpoints |
| Database Admin | http://localhost:8082 | PostgreSQL administration |
| Dashboard | http://localhost:8081/ui/dashboard.html | Visual data browser |

### Visual Dashboard
The application includes a web-based dashboard for exploring SCD data visually:

![SCD Dashboard](https://raw.githubusercontent.com/abhi14nexu/Assets/refs/heads/main/Screenshot%202025-07-27%20172134.png)

The dashboard provides:
- **Tabbed Interface**: Switch between Jobs, Timelogs, and Payments
- **Real-time Data**: Live AJAX updates from the API
- **Filtering & Search**: Find specific records quickly
- **Version History**: View all versions of entities
- **Responsive Design**: Works on desktop and mobile

## Troubleshooting

### Common Issues
1. **Port conflicts**: Ensure ports 8081, 8082, 5432 are available
2. **Database connection**: Verify PostgreSQL container is running
3. **Environment variables**: Check DATABASE_URL is set correctly

### Reset Everything
```bash
make reset
make dev-up
```

## Key Benefits

- **Developer Experience**: Simple API hides SCD complexity
- **Performance**: Optimized for temporal queries with benchmarks showing 80 µs indexed reads and 450 µs updates on a 1M-row dataset (see `make bench`)
- **Safety**: Race condition protection and data integrity
- **Flexibility**: Multiple access patterns (API, CLI, Dashboard)
- **Maintainability**: Clean architecture with comprehensive testing

## FAQ

### Q: Why is the `status` field "active" for all versions of a job?
**A**: The `status` field represents the **business status** of the job position itself, not the SCD version status. In SCD Type 2, when only certain fields change (like `rate`), the unchanged fields (like `status`) remain the same across versions. The SCD status is determined by `valid_to` - `null` means current version, non-null means historical.

### Q: How does SCD handle concurrent updates?
**A**: The SCD library uses atomic version allocation within transactions to prevent race conditions. If two updates happen simultaneously, they create separate versions with sequential version numbers. The system automatically retries on unique constraint violations.

### Q: What's the difference between `uid` and `id` fields?
**A**: 
- `uid` (UUID): Primary key that uniquely identifies each version record
- `id` (string): Business identifier that stays the same across all versions of an entity
- Example: Job "backend-dev-123" has `id="backend-dev-123"` for all versions, but each version has a unique `uid`

### Q: How do I query historical data vs current data?
**A**: Use SCD scopes:
- `scd.Latest` - Only current versions (`valid_to IS NULL`)
- `scd.Historical` - Only historical versions (`valid_to IS NOT NULL`)
- `scd.AllVersions` - All versions (current + historical)
- `scd.AsOf(time)` - Point-in-time queries

### Q: Why do I see multiple versions when I only changed one field?
**A**: SCD Type 2 creates a new version for **any** change to preserve complete audit trails. This ensures you can track exactly when each field changed, even if only one field was modified. This is the core principle of SCD Type 2.

### Q: How does the system determine which version is "latest"?
**A**: The latest version is the one with `valid_to IS NULL`. When a new version is created, the previous version's `valid_to` is set to the current timestamp, and the new version gets `valid_to = NULL`. This creates a continuous timeline of validity periods.

## License

MIT License - see [LICENSE](https://github.com/abhi14nexu/Assets/blob/main/LICENSE) file for details.

## Contributing

Contributions are welcome!
