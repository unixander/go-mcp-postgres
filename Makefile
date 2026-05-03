.PHONY: build run test lint vet tidy docker-build docker-up docker-down clean

BINARY := mcp-postgres
CMD     := ./cmd/mcp-postgres

build:
	go build -trimpath -o $(BINARY) $(CMD)

run: build
	MCP_CONFIG_PATH=config/config.yaml ./$(BINARY)

test:
	go test ./test/... -v -count=1 -timeout 120s

test-short:
	go test ./test/... -run "TestConfig|TestResolver" -v -count=1

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

docker-build:
	docker compose build

docker-up:
	docker compose up --build

docker-down:
	docker compose down

clean:
	rm -f $(BINARY)
