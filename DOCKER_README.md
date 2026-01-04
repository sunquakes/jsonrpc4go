# Docker-based Stress Testing for jsonrpc4go

This setup provides a Docker-based solution for running stress tests on the jsonrpc4go library.

## Files Included

- `Dockerfile`: Multi-stage Dockerfile that builds the HTTP server and runs stress tests
- `docker-compose.yml`: Docker Compose configuration for easy orchestration
- `test/wrk/http_docker.sh`: Docker-optimized test script
- `.dockerignore`: Specifies files to exclude from Docker context
- `test/wrk/http.lua`: Lua script for wrk to execute JSON-RPC calls

## Quick Start

1. Build and run the stress test with default parameters:
   ```bash
   docker-compose up --build
   ```

2. Or run directly with Docker:
   ```bash
   docker build -t jsonrpc-benchmark .
   docker run --rm jsonrpc-benchmark
   ```

## Custom Parameters

You can customize the stress test parameters by setting environment variables:

```bash
# With custom parameters
docker-compose run -e WRK_THREADS=8 -e WRK_CONNECTIONS=20 -e WRK_DURATION=60s jsonrpc-benchmark
```

Or with direct Docker command:
```bash
docker run --rm -e WRK_THREADS=8 -e WRK_CONNECTIONS=20 -e WRK_DURATION=60s jsonrpc-benchmark
```

Available parameters:
- `WRK_THREADS`: Number of threads (default: 4)
- `WRK_CONNECTIONS`: Number of connections (default: 10)
- `WRK_DURATION`: Duration of the test (default: 30s)

## Monitoring (Optional)

The docker-compose file includes optional monitoring services that can be enabled with profiles:

```bash
# Run with monitoring stack (Prometheus and Grafana)
docker-compose --profile monitoring up --build
```

Access Grafana at http://localhost:3000 to view metrics.

## Test Script Details

The `http_docker.sh` script:
1. Starts the HTTP server (built from examples/http/server/main.go)
2. Waits for the server to be ready
3. Runs the wrk benchmark using the http.lua script
4. Gracefully stops the server
5. Returns the exit code from wrk

## Results Interpretation

The output shows:
- Requests per second (RPS)
- Transfer rate (MB/s)
- Latency statistics
- Error rates

These metrics help evaluate the performance of the JSON-RPC server implementation.

## Troubleshooting

- If you encounter permission errors, ensure the test scripts have execute permissions
- If the server doesn't start, check that all dependencies are properly included in the Docker image
- For Windows users, the Docker setup handles CRLF line ending issues automatically

## Advanced Usage

You can also run just the server without the benchmark to test it manually:

```bash
# Run just the server
docker run --rm -p 3232:3232 --entrypoint /app/server jsonrpc-benchmark
```

Then you can make requests to http://localhost:3232 manually or with other tools.