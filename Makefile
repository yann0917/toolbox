.PHONY: build test web all clean

build:
	go build -o bin/toolbox ./cmd/toolbox

test:
	go test ./...

web:
	cd web && npm ci && npm run build
	rm -rf cmd/toolbox/webdist && mkdir -p cmd/toolbox/webdist
	cp -r web/dist/* cmd/toolbox/webdist/

all: web build

clean:
	rm -rf bin
