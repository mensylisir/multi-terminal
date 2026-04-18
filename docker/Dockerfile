# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY gateway/go.mod gateway/go.sum ./
RUN go mod download

# Copy source code
COPY gateway/ ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/server

# Frontend build
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /app/gateway /app/gateway
COPY --from=frontend-builder /app/dist /app/public

# Create non-root user
RUN addgroup -g 1000 gateway && \
    adduser -u 1000 -G gateway -s /bin/sh -D gateway

USER gateway

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
CMD ["./gateway"]