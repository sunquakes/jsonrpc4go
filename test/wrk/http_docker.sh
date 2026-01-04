#!/bin/bash

# Use the compiled server binary instead of go run
echo "Starting HTTP server..."
/app/server &
server_pid=$!
echo "Server started with PID: $server_pid"

# Wait for server to start
sleep 3

# Check if server is running properly
if ! kill -0 $server_pid 2>/dev/null; then
    echo "Failed to start server"
    exit 1
fi

echo "Server is running, starting benchmark..."
# Run wrk stress test
wrk -t4 -c10 -d30s -s /app/http.lua http://localhost:3232

# Get wrk exit code
wrk_exit_code=$?

echo "Benchmark completed, stopping server..."
# Stop server
kill $server_pid 2>/dev/null || true

# Wait for process to end
sleep 1

echo "Test completed with wrk exit code: $wrk_exit_code"
exit $wrk_exit_code