.PHONY: build test web all clean dist vet fmt

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

build:
	go build -ldflags "$(LDFLAGS)" -o bin/toolbox ./cmd/toolbox

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w cmd internal

# 构建前端并复制到 embed 目录（webdist 内只有 index.html 入库，其余产物不入库）
web:
	cd web && npm ci && npm run build
	rm -rf cmd/toolbox/webdist && mkdir -p cmd/toolbox/webdist
	cp -r web/dist/* cmd/toolbox/webdist/

all: web build

# 交叉编译发布包（纯 Go sqlite 驱动，无 CGO 依赖）
dist: web
	@rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/toolbox-$(VERSION)-$$os-$$arch"; \
		echo "building $$out"; \
		mkdir -p "$$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o "$$out/toolbox$$ext" ./cmd/toolbox || exit 1; \
		cp README.md "$$out/" 2>/dev/null || true; \
		(cd dist && zip -qr "toolbox-$(VERSION)-$$os-$$arch.zip" "toolbox-$(VERSION)-$$os-$$arch") || exit 1; \
		rm -rf "$$out"; \
	done
	@echo "产物："; ls -1 dist

clean:
	rm -rf bin dist
