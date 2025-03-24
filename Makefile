BINARY_NAME=go-hn
build:
	go build -o $(BINARY_NAME) main.go
run:
	go run main.go
clean:
	go clean
	rm -f $(BINARY_NAME)
test:
	go test ./...
dev:
	air
.PHONY: build run clean test dev
