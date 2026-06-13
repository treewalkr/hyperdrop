set shell := ["sh", "-cu"]

app := "hyperdrop"
cmd := "./cmd/hyperdrop"
packages := "./..."

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$$(gofmt -l ./cmd ./internal)"

vet:
	go vet {{packages}}

test:
	go test {{packages}}

test-race:
	go test -race {{packages}}

build:
	mkdir -p ./bin
	go build -o ./bin/{{app}} {{cmd}}

run *args:
	go run {{cmd}} {{args}}

tidy:
	go mod tidy

coverage:
	go test -coverprofile=coverage.out {{packages}}
	go tool cover -func=coverage.out

coverage-html:
	go test -coverprofile=coverage.out {{packages}}
	go tool cover -html=coverage.out -o coverage.html

check:
	just fmt-check
	just vet
	just test

clean:
	rm -rf ./bin ./coverage.out ./coverage.html
