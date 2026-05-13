# simple-web-endpoint

Just a quick little web endpoint for testing routing through complex infra scenarios.

## Usage

Run the server locally:

```sh
go run .
```

The server listens on `:8080`.

Set `LISTEN_ADDR` to bind a specific address:

```sh
LISTEN_ADDR=127.0.0.1:9090 go run .
```

Set `PORT` to bind all interfaces on a specific port:

```sh
PORT=9090 go run .
```

`LISTEN_ADDR` takes precedence when both variables are set.

## Endpoints

- `GET /` returns a small HTML page and increments a demo-only, process-local visitor count.
- `GET /healthz` returns `200 OK` after startup marks the process healthy and `503 Service Unavailable` while shutting down.

## Test

```sh
go test ./...
```

## Shutdown

The server handles `SIGINT` and `SIGTERM`, marks health checks unhealthy, and gives in-flight requests up to five seconds to finish before exiting.
