.PHONY: all dev build test test-go test-frontend lint clean

all: build

dev:
	~/go/bin/wails3 dev -config ./build/config.yml

build:
	cd frontend && npm run build
	go build -o bin/leanterm main.go

test: test-go test-frontend

test-go:
	go test -v -race ./...

test-frontend:
	cd frontend && npm test -- --run

lint:
	go vet ./...
	cd frontend && npm run check

clean:
	rm -rf bin/ dist/ frontend/dist/
