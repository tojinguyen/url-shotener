# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26 AS builder
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static, stripped binary for a minimal runtime image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server /app/server

# APP_MODE and PORT are supplied per-container by docker-compose.
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
