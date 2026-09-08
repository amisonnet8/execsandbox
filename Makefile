.PHONY: build test check race fmt fmt-check testdata cross-base

build: cross-base
	CGO_ENABLED=0 go build ./...

test:
	@for script in tests/*.sh; do \
		echo "=== $$script ==="; \
		bash "$$script" || exit 1; \
	done

check: cross-base fmt-check
	go vet ./...
	go test -count=1 ./...

fmt:
	gofmt -w .

fmt-check:
	@fmt_out="$$(gofmt -l .)"; \
	if [ -n "$$fmt_out" ]; then \
		echo "$$fmt_out" 1>&2; \
		echo "gofmt: above files are not formatted (run 'make fmt')" 1>&2; \
		exit 1; \
	fi

race: cross-base
	CGO_ENABLED=1 go test -race -count=1 ./...

# testdata/modules/*.wat の変更を testdata/modules/*.wasm へ反映する。
# .wasm はコミット対象（CIでwat2wasmを前提にしないため）なので、
# .wat を編集したら手元でこのターゲットを実行してから両方コミットすること。
testdata:
	@for wat in testdata/modules/*.wat; do \
		wat2wasm "$$wat" -o "$${wat%.wat}.wasm"; \
	done

# cmd/execsandbox-build（ビルダー）は仕様書§6.3により、自分自身に6環境分の
# ベースバイナリ（cmd/execsandboxのクロスビルド成果物）を//go:embedで内包する。
# このターゲットはその埋め込み対象を用意する。生成物はコミットしない
# （.gitignore）ため、cmd/execsandbox-buildパッケージをgo build/go vet/
# go testする前に必ず実行しておく必要がある（build/check/raceが依存している
# 理由）。CGO_ENABLED=0のためcrosコンパイルにCコンパイラは不要。
cross-base:
	@mkdir -p cmd/execsandbox-build/basebinaries
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_linux_amd64     ./cmd/execsandbox
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_linux_arm64     ./cmd/execsandbox
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_darwin_amd64    ./cmd/execsandbox
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_darwin_arm64    ./cmd/execsandbox
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_windows_amd64.exe ./cmd/execsandbox
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o cmd/execsandbox-build/basebinaries/execsandbox_windows_arm64.exe ./cmd/execsandbox
