.PHONY: run build test lint migrate-up migrate-down migrate-new docker-up docker-down docker-build swagger clean

run:
	@if command -v air > /dev/null; then \
		air; \
	else \
		go run cmd/server/main.go; \
	fi

build:
	go build -o tmp/main cmd/server/main.go

test:
	go test -v ./...

lint:
	golangci-lint run

migrate-up:
	goose -dir migrations postgres "user=postgres password=postgres dbname=nwstep sslmode=disable" up

migrate-down:
	goose -dir migrations postgres "user=postgres password=postgres dbname=nwstep sslmode=disable" down

migrate-new:
	@read -p "Enter migration name: " name; \
	goose -dir migrations create $$name sql

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose up --build -d

swagger:
	swag init -g cmd/server/main.go -o docs

clean:
	rm -rf tmp/ docs/
