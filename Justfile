set shell := ["sh", "-cu"]

app := "hyperdrop"
cmd := "./cmd/hyperdrop"
packages := "./..."
version_pkg := "github.com/treewalkr/hyperdrop/internal/version"
git_commit := `git rev-parse --short HEAD 2>/dev/null || echo none`
ldflags := "-s -w -X {{version_pkg}}.Version=dev -X {{version_pkg}}.Commit={{git_commit}} -X {{version_pkg}}.Date=unknown"

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
	go build -ldflags "{{ldflags}}" -o ./bin/{{app}} {{cmd}}

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
