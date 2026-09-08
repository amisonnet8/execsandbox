.PHONY: build test check race fmt testdata

build:
	CGO_ENABLED=0 go build ./...

test:
	@for script in tests/*.sh; do \
		echo "=== $$script ==="; \
		bash "$$script" || exit 1; \
	done

check: fmt-check
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

race:
	CGO_ENABLED=1 go test -race -count=1 ./...

# testdata/modules/*.wat の変更を testdata/modules/*.wasm へ反映する。
# .wasm はコミット対象（CIでwat2wasmを前提にしないため）なので、
# .wat を編集したら手元でこのターゲットを実行してから両方コミットすること。
testdata:
	@for wat in testdata/modules/*.wat; do \
		wat2wasm "$$wat" -o "$${wat%.wat}.wasm"; \
	done
