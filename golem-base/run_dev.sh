#!/usr/bin/env bash

# This scipt will build/run the Golem base node in development mode.
# It will run the node with the eth,web3,net,debug and golembase http APIs enabled.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -e "/tmp/golembase.wal" ]]; then
    if [[ -d "/tmp/golembase.wal" ]]; then
        rm -rf "/tmp/golembase.wal"
    elif [[ -f "/tmp/golembase.wal" ]]; then
        rm "/tmp/golembase.wal"
    fi
fi

exec go run "$SCRIPT_DIR/../cmd/geth" --dev \
    --http --http.api "eth,web3,net,debug,golembase" \
    --verbosity 3 \
    --http.addr "0.0.0.0" \
    --http.port 8545 \
    --http.corsdomain "*" \
    --http.vhosts "*" \
    --golembase.writeaheadlog "/tmp/golembase.wal"
