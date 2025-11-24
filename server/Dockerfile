# Multi-stage build for Go monitoring server

FROM golang:1.24.5-alpine AS builder

WORKDIR /build/server

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY ./server/go.mod ./server/go.sum ./
RUN go mod download

# Copy source code
COPY ./server .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /build/monitoring-server .

# Final stage
FROM alpine:3.22.2

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/monitoring-server .

# Copy static files and templates
COPY ./web ./web

# Create log directory
RUN mkdir -p /app/logs

# Expose port
EXPOSE 8080

# Run the server
CMD ["./monitoring-server"]
