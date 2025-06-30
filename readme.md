
# API-Gateway: A scaffold to build a custom API-Gateway

no bs - a minimalistic, extensible API Gateway for your next application.

Need a single, reliable point of entry for your microservices? API-Gateway provides the core, essential features you need to secure and manage traffic to your backend services without the bloat of enterprise solutions. It's built with a middleware-first approach, making it easy to extend and customize.

## Features

*   **⚡ Dynamic Routing:** Route incoming requests to the appropriate downstream service based on a simple YAML configuration file. No hardcoded routes, no redeploys needed to change routing logic.

*   **🛡️ JWT Authentication:** Secure your endpoints with JSON Web Tokens. (Middleware included).

*   **🚦 Rate Limiting:** Protect your services from abuse and ensure fair usage with a configurable token-bucket rate limiter. Apply limits per client IP address.

*   **📝 Centralized Logging:** Gain insight into your traffic with structured (JSON) logs for every request, including latency, status code, and a unique request ID.

*   **🔗 Middleware Chaining:** Easily add custom functionality to the request/response lifecycle. All core features are implemented as plug-and-play middleware.

*   **🔄 Request/Response Transformation:** Modify headers on the fly. Automatically adds an `X-Request-ID` header for end-to-end traceability.

*   ** graceful-shutdown Graceful Shutdown:** Ensures no in-flight requests are dropped when the server needs to restart or shut down, making it safe for production environments.

## Getting Started

### Prerequisites

- Go 1.18+

### Installation & Running

1.  **Clone the repository:**
    ```sh
    git clone https://github.com/Gen1us1100/api-gateway.git
    cd api-gateway
    ```

2.  **Configure your routes:**
    Create a `config.yaml` file in the root of the project. See the [Configuration](#configuration) section below for an example.

3.  **Run the application:**
    ```sh
    go run ./cmd/api
    ```
    By default, the API Gateway will start on port `:8000`.

## Configuration

The gateway is controlled by a `config.yaml` file. Here is an example demonstrating how to define the server port and your service routes.

```yaml
# config.yaml
server:
  port: 8000

# A list of all routes the gateway will manage.
routes:
  - path: "/api/users/" # Incoming path prefix
    method: "GET"
    # The backend service to forward the request to.
    upstream_url: "http://user-service:3001"

  - path: "/api/orders/"
    method: "POST"
    upstream_url: "http://order-service:3002"

  - path: "/api/orders/"
    method: "GET"
    upstream_url: "http://order-service:3002"

```

The gateway will match an incoming request based on its `path` prefix and `method`, then proxy it to the corresponding `upstream_url`.

## Project Structure

The project follows a standard Go project layout to separate concerns.

```
.
├── cmd/
│   └── api/           # Main application entry point. Wires everything together.
├── internal/
│   ├── models/        # Data models (e.g., config structs).
│   ├── handlers/      # The core reverse proxy handler.
│   ├── services/      # Business logic (e.g., rate limiting algorithm).
│   └── repository/    # Data access layer (e.g., loading config from file).
├── pkg/
│   ├── config/        # Configuration management structs and loaders.
│   └── middleware/    # HTTP middleware (Auth, Logging, Rate Limiting, etc.).
└── docs/             # Documentation.
```

## Architectural Philosophy

*   **Simplicity:** The core proxying logic is minimal, leveraging Go's robust `net/http/httputil` package.
*   **Middleware-First:** All cross-cutting concerns like logging, authentication, and rate-limiting are implemented as a chain of standard `http.Handler` middleware. This makes the system easy to reason about and extend.
*   **Configuration-Driven:** There are no hardcoded routes or policies in the Go code. The gateway's behavior is defined entirely by the external `config.yaml` file, promoting flexibility.

## Contributing

Contributions are welcome! Please feel free to submit a pull request.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

