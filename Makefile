.PHONY: all build-wasm copy-wasm run clean

all: build-wasm copy-wasm run

build-wasm:
	GOOS=js GOARCH=wasm go build -o client/sim.wasm ./wasm/

copy-wasm:
	cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" client/

run:
	go run main.go

clean:
	rm -f client/sim.wasm client/wasm_exec.js