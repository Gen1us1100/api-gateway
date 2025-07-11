# Go-Gateway: A Lightweight, Production-Ready Gateway in Go

A no-bs, minimalistic, and high-performance API Gateway scaffold built in Go. This project provides a single, secure entry point for your microservices, complete with essential production-grade features. It's designed to be simple to understand, configure, and extend.

## Why Use This Gateway?

*   **Centralized Control:** Manage cross-cutting concerns like authentication, rate limiting, and logging in one place.
*   **Secure by Default:** Implements modern security best practices out-of-the-box.
*   **Highly Observable:** Structured, request-correlated logging provides deep insight into your traffic.
*   **Simple & Extendable:** Built with standard Go libraries and a clean middleware pattern, making it easy to add your own custom logic.

## Features

-   **Dynamic Routing:** Route requests to different backend services based on a simple YAML configuration. No need to recompile to add a new service.
-   **JWT Authentication:** Secure your routes with JSON Web Tokens. The gateway validates the token and passes the user's identity to upstream services.
-   **Rate Limiting:** Protect your services from abuse with a per-IP, token-bucket rate limiter.
-   **Structured Logging:** Rich, structured (JSON) logs for every request, including a unique `request_id` for easy tracing.
-   **Advanced Observability:** Measures and logs both total request latency and the specific latency of upstream service calls, helping you pinpoint bottlenecks instantly.
-   **Request/Response Transformation:** Automatically adds security headers (`X-Content-Type-Options`, `X-Frame-Options`, etc.) to every response and propagates context like `X-Request-ID` and `X-User-ID` to your backend services.
-   **Graceful Shutdown:** Ensures no in-flight requests are dropped during a restart or deployment.
-   **CORS Handling:** Centralized and configurable CORS handling at the edge.


## Getting Started

### Prerequisites

-   Go 1.18+
-   Docker & Docker Compose (Recommended for easy database setup)
-   [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate): A CLI tool for running database migrations.

You can install `golang-migrate` with Go:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```
Make sure your Go `bin` directory is in your system's `PATH`.

### Installation & Setup

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/Gen1us1100/go-gateway.git
    cd go-gateway
    # for resolving dependencies
    go mod tidy
    ```

2.  **Configure your environment:**
    Create a `.env` file in the root of the project. This file is for local development secrets and should **not** be committed to Git. A `.env.example` file is provided for reference.
    ```ini
    # .env
    DB_USER="myuser"
    DB_PASSWORD="your-postgres-password"
    DB_HOST="localhost"
    DB_PORT="5432"
    DB_NAME="apigateway"
    JWT_SECRET="a-very-long-and-secure-random-string"
    ```

3.  **Start a PostgreSQL Database:**
    The easiest way to get a local PostgreSQL instance running is with Docker Compose. A `docker-compose.yml` file can be used for this (you can add one to your project).
    ```bash
    # (If you have a docker-compose.yml)
    docker-compose up -d
    ```
    Alternatively, ensure you have a local PostgreSQL server running that matches the credentials in your `.env` file.

4.  **Run Database Migrations:**
    This step creates the necessary `users` table for authentication to work. See the "Database Migrations" section below for details.
    ```bash
    make migrate-up
    ```
    *(We'll define this `make` command in the next section).*

5.  **Configure your routes:**
    Modify the `config.yaml` file to define your services and routing rules. The gateway strips the `/api` prefix, so define paths relative to that.

    ```yaml
    # config.yaml
    port: "8080"
    
    db_host: "localhost"
    db_port: "5432"
    db_user: "myuser"
    db_name: "mydatabase"

    routes:
      # Requests to /api/users/* will go to the user-service
      - path_prefix: "/users"
        upstream_url: "http://localhost:8081"
        
      # Requests to /api/orders/* will go to the order-service
      - path_prefix: "/orders"
        upstream_url: "http://localhost:8082"
        
      # A catch-all for any other /api/* path
      - path_prefix: "/"
        upstream_url: "http://localhost:8083"

6.  **Run the gateway:**
    ```bash
    go run ./cmd/api/main.go
    ```
    The server will now be running on port 8080!

---

## Database Migrations

This project uses `golang-migrate` to manage database schema changes. The migration files are located in the `/migrations` directory.

### Creating the `users` Table

1.  **Create the migration file:**
    Inside the `/migrations` directory, create a new file named `000001_create_users_table.up.sql`.

2.  **Add the SQL content:**
    Paste the following SQL into the file you just created:
    ```sql
    -- /migrations/000001_create_users_table.up.sql
    CREATE TABLE IF NOT EXISTS users (
        id UUID PRIMARY KEY,
        user_name VARCHAR(100) NOT NULL,
        email VARCHAR(255) NOT NULL UNIQUE,
        password VARCHAR(255) NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );
    ```

### Running Migrations

To make running migrations easy, you can add commands to a `Makefile` in your project root.

**Create a `Makefile`:**
```makefile
# Makefile

# Load environment variables from .env file
include .env
export

# Construct the database URL from environment variables
# Note: Use sslmode=disable for local development without SSL.
DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable

.PHONY: migrate-up migrate-down

migrate-up:
	@echo "Running migrations up..."
	migrate -path migrations -database "${DATABASE_URL}" -verbose up

migrate-down:
	@echo "Running migrations down..."
	migrate -path migrations -database "${DATABASE_URL}" -verbose down

```

Now, you can simply run `make migrate-up` from your terminal, and it will apply all pending migrations to your database.


## Project Structure

```
.
├── cmd/
│   └── api/           # Main application entry point
├── internal/
│   ├── models/        # Data models (e.g. User)
│   ├── handlers/      # Core proxy and user auth handlers
│   ├── services/      # Business logic (e.g. Rate Limiter state)
│   └── repository/    # Data access layer (e.g. database interactions)
├── pkg/
│   ├── config/        # Configuration management (YAML + .env)
│   ├── db/            # Database connection setup
│   └── middleware/    # HTTP middleware (Auth, Logging, Rate Limiting, etc.)
└── docs/             # Documentation
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request
