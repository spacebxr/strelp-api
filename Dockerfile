# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both binaries
RUN go build -o bot ./cmd/bot
RUN go build -o api ./cmd/api

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bot .
COPY --from=builder /app/api .
COPY --from=builder /app/migrations ./migrations

# Expose the API port
EXPOSE 8080

# The default command (can be overridden in Railway)
CMD ["./bot"]
