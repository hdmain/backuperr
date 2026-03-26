#!/bin/bash

echo "Building client..."
go build -o client ./cmd/client

echo "Building host..."
go build -o host ./cmd/host

echo "Sending files via tcpraw..."
tcpraw send -server=1 host
tcpraw send -server=1 client
echo "Done."