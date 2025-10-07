# Infrastream Security Demo Review Service

Product review and rating management service for Infrastream Security Demo platform.

## Quick Start

```bash
go mod download
go run main.go
```

## Build

```bash
go build -o svc-review
```

## Configuration

The service can be configured via a YAML file, environment variables, or a combination of both. Environment variables will always override values set in the configuration file.

### Configuration File

An example configuration file, `config.yaml.example`, is provided. You can copy it to `config.yaml` and modify it to suit your needs. To use a configuration file, set the `CONFIG_FILE` environment variable to its path.

### Environment Variables

All configuration options can be set using environment variables with the `SVC_` prefix. For example, to set the port, you would use `SVC_PORT=8081`.

### Configuration Options

| Name          | YAML Key      | Environment Variable | Default     | Description                                                                 |
|---------------|---------------|----------------------|-------------|-----------------------------------------------------------------------------|
| Port          | `port`        | `SVC_PORT`           | `8080`      | The port the service will listen on.                                        |
| Database URL  | `database_url`| `SVC_DATABASE_URL`   | `""`        | The full DSN for the database connection. If provided, it takes precedence. |
| DB Host       | `db_host`     | `SVC_DB_HOST`        | `localhost` | The database host.                                                          |
| DB Port       | `db_port`     | `SVC_DB_PORT`        | `5432`      | The database port.                                                          |
| DB User       | `db_user`     | `SVC_DB_USER`        | `postgres`  | The database user.                                                          |
| DB Password   | `db_pass`     | `SVC_DB_PASS`        | `postgres`  | The database password.                                                      |
| DB Name       | `db_name`     | `SVC_DB_NAME`        | `product_catalog` | The database name.                                                      |
| Redis Host    | `redis_host`  | `SVC_REDIS_HOST`     | `localhost` | The Redis host.                                                             |
| Redis Port    | `redis_port`  | `SVC_REDIS_PORT`     | `6379`      | The Redis port.                                                             |

## Requirements

- Go 1.21+
- PostgreSQL 14+