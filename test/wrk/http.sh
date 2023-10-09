#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SCRIPT_NAME="http.sh"
SERVER_PATH="$SCRIPT_DIR/../../examples/http/server/main.go"
SCRIPT_PATH="$SCRIPT_DIR/http.lua"

go run "$SERVER_PATH" &
server_pid=$!
sleep 3
wrk -t4 -c10 -d30s -s "$SCRIPT_PATH" http://localhost:3232
kill $server_pid