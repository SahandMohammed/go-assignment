# Wallet Service Backend

A simple but complete wallet backend service built with Go, Gin, GORM, MySQL, and Redis.

## Features

- User authentication with JWT tokens
- Wallet creation and management
- Deposit functionality
- Transfer between wallets with atomic transactions
- Transaction history
- Admin APIs for users and transactions
- Redis caching for performance
- Comprehensive logging and audit trail
- Money stored in minor units (cents) for precision

## Input & Business Validation

| Domain    | Rule                | Details                                                                                          |
| --------- | ------------------- | ------------------------------------------------------------------------------------------------ |
| Username  | Alphabetic only     | Registration rejects any username containing digits, spaces, or symbols (e.g. `user123` -> 400). |
| Password  | Length 8–15         | Enforced via validation tags; shorter or longer values rejected.                                 |
| Amounts   | Positive & non-zero | Deposits > 0; transfers > 0; negative/zero rejected with 400.                                    |
| Ownership | Access control      | Users can only act on their own wallets (403 on cross‑wallet access).                            |
| JWT       | Required            | All protected endpoints require a valid Bearer token.                                            |
| Transfer  | Sufficient funds    | Insufficient balance returns 400 with explanatory error.                                         |

All validations are tested automatically by the bundled `test_api.sh` script.

## Prerequisites

- Go 1.21+
- Docker and Docker Compose

## Quick Start

1. **Clone and setup the project:**

   ```bash
   git clone <repository-url>
   cd wallet-service
   ```

2. **Start the infrastructure:**

   ```bash
   docker compose -f docker/docker-compose.yml up -d
   ```

3. **Run the application:**

   ```bash
   go run ./cmd/server
   ```

4. **Test the health endpoint:**
   ```bash
   curl http://localhost:8080/health
   ```

## API Endpoints

### Authentication

- `POST /auth/register` - Register a new user
- `POST /auth/login` - Login and get JWT token

### Wallet Management (Protected)

- `POST /wallets` - Create a new wallet
- `GET /wallets` - Get user's wallets
- `GET /wallets/:id` - Get specific wallet
- `POST /wallets/deposit` - Deposit money to wallet
- `POST /wallets/transfer` - Transfer money between wallets
- `GET /wallets/:id/transactions` - Get wallet transactions

### Admin APIs (Protected)

- `GET /admin/users` - List all users and their wallets
- `GET /admin/transactions` - List transactions with filters

## Example API Usage

### 1. Register a user

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "password123"}'
```

### 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "password123"}'
```

### 3. Create a wallet (use the token from login)

```bash
curl -X POST http://localhost:8080/wallets \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### 4. Deposit money

```bash
curl -X POST http://localhost:8080/wallets/deposit \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"wallet_id": 1, "amount": 100.50, "description": "Initial deposit"}'
```

### 5. Transfer money

```bash
curl -X POST http://localhost:8080/wallets/transfer \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"from_wallet_id": 1, "to_wallet_id": 2, "amount": 25.00, "description": "Transfer to friend"}'
```

## Configuration

Environment variables can be set in `.env` file:

```env
APP_ENV=development
APP_PORT=8080
APP_JWT_SECRET=supersecret_change_me

MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USER=wallet
MYSQL_PASSWORD=walletpw
MYSQL_DB=walletdb

REDIS_ADDR=127.0.0.1:6379
REDIS_DB=0
REDIS_PASSWORD=

LOG_LEVEL=info
```

## Development

### Project Structure

```
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── db/              # Database connections
│   ├── domain/          # Domain models
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic
│   ├── http/
│   │   ├── handler/     # HTTP handlers
│   │   ├── middleware/  # HTTP middleware
│   │   └── router/      # Route setup
│   └── migration/       # Database migrations
├── docker/              # Docker configuration
└── configs/             # Configuration files
```

## Security Considerations

- Passwords are hashed using bcrypt
- JWT tokens expire after 24 hours
- All financial operations are protected by authentication
- Database transactions ensure data consistency
- Input validation on all endpoints

## Testing

### End-to-End API Test Script

An executable bash script `test_api.sh` performs a full black‑box verification of the service (health, auth, validation, wallets, deposits, transfers, admin, security, Redis, error cases).

Quick run:

```bash
bash test_api.sh
```

What it does:

1. Verifies health/ready/live endpoints
2. (Re)registers sender & receiver test users (idempotent)
3. Exercises validation failures (invalid username, short password, bad credentials)
4. Logs in and captures JWT tokens
5. Creates wallets (if not already present)
6. Performs deposits & negative/unauthorized deposit tests
7. Executes valid, insufficient funds, and unauthorized transfers
8. Checks balances & transaction history with pagination
9. Calls admin listing endpoints
10. Validates security scenarios (invalid / missing token, cross-user access)
11. Optionally inspects Redis keys (if accessible via Docker)
12. Runs error handling edge cases (zero amount, non-existent wallet)

Requirements:

- `curl` (mandatory)
- `bash`
- `docker` (optional, only needed for direct Redis key inspection)

Exit codes:

- `0` success (all checks passed)
- Non-zero: the first failing step prints a contextual error message

### Troubleshooting the Script

| Symptom                         | Cause                                                  | Fix                                                        |
| ------------------------------- | ------------------------------------------------------ | ---------------------------------------------------------- |
| Script stops at validation step | `set -e` causing exit due to earlier unhandled command | Re-run; recent version guards expected failures.           |
| Raw JSON not pretty printed     | `jq` missing                                           | Install `jq` or ignore (optional).                         |
| Redis section shows 0 keys      | Container name differs / not running                   | Ensure `wallet-redis` container name matches compose file. |
| 401 on protected endpoints      | Missing / expired JWT                                  | Re-run script to refresh tokens or verify system clock.    |
| 403 on wallet actions           | Using other user's token intentionally                 | Expected security behavior.                                |

### Redis Caching Notes

User objects are cached after login; database remains the source of truth (DB-first, then cache populate). The test script does not rely on cache presence to succeed, ensuring deterministic behavior.

## Docker Support

The project includes Docker Compose configuration for MySQL and Redis:

```bash
# Start infrastructure
docker compose -f docker/docker-compose.yml up -d

# View logs
docker compose -f docker/docker-compose.yml logs -f

# Stop and clean up
docker compose -f docker/docker-compose.yml down -v
```
