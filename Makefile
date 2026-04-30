.PHONY: all build-wasm copy-wasm run run-local build-linux deploy setup-server clean build-notifier setup-notifier deploy-notifier

# ----- local dev -------------------------------------------------------

all: build-wasm copy-wasm run

build-wasm:
	GOOS=js GOARCH=wasm go build -o client/sim.wasm ./wasm/

copy-wasm:
	cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" client/

# embed requires sim.wasm + wasm_exec.js to be present at build time
run: build-wasm copy-wasm
	PORT=8088 go run main.go

# Server-side simulation with Go concurrency — local development only, not deployed.
# Flags: -n (particles), -steps (0=infinite), -dt, -workers, -interval, -box
# Example: make run-local ARGS="-n 5000 -steps 1000 -workers 8"
run-local:
	go run ./cmd/localsim/ $(ARGS)

# ----- deploy ----------------------------------------------------------

DEPLOY_HOST ?= gsim.vdisknow.com
DEPLOY_USER ?= root
DEPLOY_DIR  ?= /opt/grav-charge-sim
BINARY      := grav-charge-sim

# Cross-compile a self-contained Linux/amd64 binary with all client
# files embedded — nothing else needs to exist on the server.
build-linux: build-wasm copy-wasm
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) .

# Build, push binary to server, restart service. Run for every update.
# Uploads to a .new temp file first so scp never touches the running binary,
# then stop/swap/start in one ssh call to minimise downtime.
deploy: build-linux
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "mkdir -p $(DEPLOY_DIR)"
	scp $(BINARY) $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_DIR)/$(BINARY).new
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "systemctl stop grav-charge-sim && mv $(DEPLOY_DIR)/$(BINARY).new $(DEPLOY_DIR)/$(BINARY) && systemctl start grav-charge-sim"
	rm -f $(BINARY)
	@echo "Deployed to https://gsim.vdisknow.com"

# One-time setup on a fresh Debian/Ubuntu droplet.
# Creates the app directory, installs the systemd unit, and enables it.
# The DO load balancer forwards :80 → droplet:8088 directly — no nginx needed.
# Run once, then use `make deploy` for all subsequent updates.
setup-server:
	scp deploy/grav-charge-sim.service \
		$(DEPLOY_USER)@$(DEPLOY_HOST):/etc/systemd/system/grav-charge-sim.service
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) " \
		mkdir -p $(DEPLOY_DIR) && \
		mkdir -p $(DEPLOY_DIR)/certs && \
		systemctl daemon-reload && \
		systemctl enable grav-charge-sim"
	@echo "Server ready. Run 'make deploy' to push the first binary."

# ----- notifier --------------------------------------------------------

NOTIFIER_HOST ?= gsim.vdisknow.com
NOTIFIER_USER ?= root
NOTIFIER_DIR  ?= /opt/grav-charge-notifier
NOTIFIER_BIN  := grav-charge-notifier

build-notifier:
	GOOS=linux GOARCH=amd64 go build -o $(NOTIFIER_BIN) ./notifier/

# One-time setup: install the config, copy the service unit, enable it.
setup-notifier:
	ssh $(NOTIFIER_USER)@$(NOTIFIER_HOST) "mkdir -p $(NOTIFIER_DIR)"
	scp notifier/config.json $(NOTIFIER_USER)@$(NOTIFIER_HOST):$(NOTIFIER_DIR)/config.json
	scp deploy/grav-charge-notifier.service \
		$(NOTIFIER_USER)@$(NOTIFIER_HOST):/etc/systemd/system/grav-charge-notifier.service
	ssh $(NOTIFIER_USER)@$(NOTIFIER_HOST) " \
		systemctl daemon-reload && \
		systemctl enable grav-charge-notifier"
	@echo "Notifier service installed. Run 'make deploy-notifier' to push the first binary."

deploy-notifier: build-notifier
	scp $(NOTIFIER_BIN) $(NOTIFIER_USER)@$(NOTIFIER_HOST):$(NOTIFIER_DIR)/$(NOTIFIER_BIN).new
	ssh $(NOTIFIER_USER)@$(NOTIFIER_HOST) " \
		systemctl stop grav-charge-notifier 2>/dev/null || true && \
		mv $(NOTIFIER_DIR)/$(NOTIFIER_BIN).new $(NOTIFIER_DIR)/$(NOTIFIER_BIN) && \
		systemctl start grav-charge-notifier"
	rm -f $(NOTIFIER_BIN)
	@echo "Notifier deployed."

# ----- clean -----------------------------------------------------------

clean:
	rm -f client/sim.wasm client/wasm_exec.js $(BINARY)
