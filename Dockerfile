# Use official Go image as build environment
FROM golang:1.24-alpine AS builder

# Install wrk stress testing tool and dependencies
RUN apk add --no-cache git curl bash wrk

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy only necessary source code directories instead of everything
COPY client/ ./client/
COPY common/ ./common/
COPY discovery/ ./discovery/
COPY server/ ./server/
COPY examples/ ./examples/
COPY client.go server.go ./

# Build HTTP server to temporary directory
RUN mkdir -p /tmp/build && go build -o /tmp/build/server examples/http/server/main.go

# Create final runtime image
FROM golang:1.24-alpine

# Install runtime dependencies and create non-root user
RUN apk add --no-cache wrk bash curl && \
    addgroup -g 1001 appuser && \
    adduser -S -u 1001 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy compiled server from build stage
COPY --from=builder /tmp/build/server /app/server

# Copy test scripts and configuration
COPY test/wrk/http_docker.sh /app/test.sh
COPY test/wrk/http.lua /app/http.lua

# Use sed to fix line ending issues (handle Windows CRLF)
RUN sed -i 's/\r$//' /app/test.sh

# Set proper permissions for the application files
RUN chown -R appuser:appuser /app && \
    chmod +x /app/test.sh /app/server

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3232

# Run stress test
CMD ["/bin/bash", "-c", "/app/test.sh"]