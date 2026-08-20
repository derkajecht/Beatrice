# build main
build:
    go build -o client ./cmd/client
    go build -o server ./cmd/server

client:
    go run ./cmd/client --host localhost --port 8080

server:
    go run ./cmd/server --host localhost --port 8080
