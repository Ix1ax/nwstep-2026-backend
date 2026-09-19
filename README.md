# NWSTEP Hackathon 2026 API

Complete Go Fiber v2 boilerplate with Modular Monolith architecture.

## Quick Start
```bash
make docker-up
```

## Structure
- `cmd/server`: Application entrypoint
- `internal/`: Domain modules (auth, user, ws, upload)
- `pkg/`: Shared utilities (config, database, logger, middleware, response, storage)
- `migrations/`: Database migrations

## API Endpoints
| Method | Path | Description | Protected |
|---|---|---|---|
| POST | `/api/v1/auth/register` | Register user | No |
| POST | `/api/v1/auth/login` | Login user | No |
| POST | `/api/v1/auth/refresh` | Refresh token | No |
| GET | `/api/v1/auth/me` | Current user | Yes |
| GET | `/api/v1/users` | List users | No |
| GET | `/api/v1/users/:id` | Get user | No |
| PUT | `/api/v1/users/:id` | Update user | Yes |
| GET | `/api/v1/ws` | WebSocket | Token query |
| POST | `/api/v1/upload` | Upload file | Yes |
| GET | `/api/v1/upload/:id` | Get file URL | No |
| DELETE | `/api/v1/upload/:id` | Delete file | Yes |

## Deployment
Use `docker-compose.prod.yml` for production deployment.
