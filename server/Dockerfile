# Multi-stage build for Go monitoring server

FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o monitoring-server .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/monitoring-server .

# Copy static files and templates
COPY --from=builder /build/web ./web

# Create log directory
RUN mkdir -p /app/logs

# Expose port
EXPOSE 5000

# Run the server
CMD ["./monitoring-server"]
