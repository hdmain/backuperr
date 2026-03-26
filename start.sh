#!/bin/bash

# Budowanie programów Go
echo "Building client..."
go build -o client ./cmd/client

echo "Building host..."
go build -o host ./cmd/host

# Wysyłanie plików przez tcpraw
# Upewnij się, że 'host' jest odpowiednim argumentem dla tcpraw
# Możesz zmienić 'host' na ścieżkę/alias serwera
echo "Sending files via tcpraw..."
tcpraw send -server=1 host
tcpraw send -server=1 client
echo "Done."