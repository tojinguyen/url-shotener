# url-shortener

A horizontally-scalable URL shortener written in Go. The system is split into two
independently scalable paths that ship as **one binary** selected at runtime via the
`APP_MODE` environment variable, and run as **separate containers** from the same image:

- **Shortener (write path)** — `POST /api/v1/shorten`: hashes the long URL, generates a unique
  7-character key, persists it, and seeds the cache.
- **Redirector (read path)** — `GET /{short_url}`: resolves the key via a cache-aside lookup and
  issues an HTTP 302 redirect.

## Stack

| Concern        | Technology                                   |
| -------------- | -------------------------------------------- |
| Language       | Go 1.26                                       |
| Router         | `go-chi/chi/v5`                               |
| Storage        | MongoDB (unique index + TTL index)           |
| Cache          | Redis (cache-aside, `SETEX`)                 |
| Collision filter | In-memory Bloom filter (`bits-and-blooms/bloom`) |

## Architecture

```
                 POST /api/v1/shorten                 GET /{short_url}
                        │                                    │
                 ┌──────▼───────┐                     ┌──────▼───────┐
                 │  Shortener   │                     │  Redirector  │
                 │  (write)     │                     │  (read)      │
                 └──┬───┬───┬───┘                     └────┬────┬────┘
        Bloom probe │   │   │ SETEX                   GET  │    │ SETEX (lazy)
                    │   │   ▼                              ▼    │
                    │   │  Redis  ◄───────────────────────┘    │
                    │   ▼                                       │
                    │  MongoDB (urls: unique short_url, TTL expire_at) ◄┘
                    ▼
              In-memory Bloom filter (warmed from Mongo on boot)
```

### Key generation (write path)
1. `expireAt = now + DEFAULT_TTL_DAYS`.
2. Attempt 0: `Base62(MD5(long_url))[:7]`. Retries: salt with `UnixNano` before hashing.
3. Probe the Bloom filter: a negative is conclusive (key is free). A positive triggers a
   confirming `FindOne` against MongoDB; if a doc exists, retry (up to 10 times, else HTTP 500).
4. Insert the document, `SETEX` the cache for the full TTL, and add the key to the Bloom filter.

### Resolution (read path)
1. Redis hit → 302 immediately.
2. Miss → MongoDB `FindOne`; missing → 404.
3. Found but past `expire_at` (guards against TTL-sweep lag) → 404.
4. Valid → lazily `SETEX` for the remaining lifetime → 302.

## Project layout

```
cmd/server/        # entrypoint; reads APP_MODE, wires dependencies
internal/config/   # environment-driven configuration
internal/model/    # URL document + request/response DTOs
internal/store/    # Mongo client + indexes, Redis cache, repository
internal/service/  # Shortener and Redirector business logic
internal/handler/  # chi handlers + per-mode router
internal/util/     # MD5, Base62, Bloom filter helpers
```

## Running

### Docker Compose (recommended)
```bash
cp .env.example .env   # optional: tweak DEFAULT_TTL_DAYS, bloom sizing
docker compose up --build
```
This starts MongoDB, Redis, the **shortener** on `:8080`, and the **redirector** on `:8081`.
Scale either path independently:
```bash
docker compose up --build --scale shortener=3 --scale redirector=5
```

### Local (single process, both paths)
```bash
APP_MODE=all go run ./cmd/server   # needs local MongoDB + Redis
```

## API

### Create a short URL
```bash
curl -i -X POST localhost:8080/api/v1/shorten \
  -H 'Content-Type: application/json' \
  -d '{"long_url":"https://example.com/some/very/long/path"}'
```
```
HTTP/1.1 201 Created
{"short_url":"aZ3xK9b","long_url":"https://example.com/some/very/long/path","expire_at":"2026-07-15T..."}
```

### Follow a short URL
```bash
curl -i localhost:8081/aZ3xK9b
```
```
HTTP/1.1 302 Found
Location: https://example.com/some/very/long/path
```
Unknown or expired keys return `404 Not Found`.

## Configuration

| Variable           | Default                     | Notes                                  |
| ------------------ | --------------------------- | -------------------------------------- |
| `APP_MODE`         | `all`                       | `shortener` \| `redirector` \| `all`   |
| `PORT`             | `8080`                      | HTTP listen port                       |
| `MONGO_URI`        | `mongodb://localhost:27017` |                                        |
| `MONGO_DB`         | `urlshortener`              |                                        |
| `REDIS_ADDR`       | `localhost:6379`            |                                        |
| `REDIS_PASSWORD`   | _(empty)_                   |                                        |
| `REDIS_DB`         | `0`                         |                                        |
| `DEFAULT_TTL_DAYS` | `30`                        | Expiry applied to every new URL        |
| `BLOOM_CAPACITY`   | `10000000`                  | Expected key count (shortener/all)     |
| `BLOOM_FP_RATE`    | `0.001`                     | Target false-positive rate             |

## Testing
```bash
go test ./...
```
Unit tests cover the deterministic Base62 conversion, the collision retry loop (mocked Mongo /
Redis / Bloom), and the cache-aside resolution paths (hit, miss→DB, expired, missing).
