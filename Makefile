.PHONY: build test web all clean

build:
	go build -o bin/toolbox ./cmd/toolbox

test:
	go test ./...

web:
	cd web && npm ci && npm run build

all: build

clean:
	rm -rf bin
