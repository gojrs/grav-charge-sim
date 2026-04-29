.PHONY: all build-wasm copy-wasm run build-linux deploy setup-server clean

# ----- local dev -------------------------------------------------------

all: build-wasm copy-wasm run

build-wasm:
	GOOS=js GOARCH=wasm go build -o client/sim.wasm ./wasm/

copy-wasm:
	cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" client/

# embed requires sim.wasm + wasm_exec.js to be present at build time
run: build-wasm copy-wasm
	go run main.go

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
deploy: build-linux
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "mkdir -p $(DEPLOY_DIR)"
	scp $(BINARY) $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_DIR)/$(BINARY)
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "systemctl restart grav-charge-sim"
	rm -f $(BINARY)
	@echo "Deployed to http://gsim.vdisknow.com"

# One-time setup on a fresh Debian/Ubuntu droplet.
# Installs nginx, drops service + nginx configs, enables everything.
# Run once, then use `make deploy` for all subsequent updates.
setup-server:
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "apt-get update -qq && apt-get install -y -qq nginx"
	scp deploy/grav-charge-sim.service \
		$(DEPLOY_USER)@$(DEPLOY_HOST):/etc/systemd/system/grav-charge-sim.service
	scp deploy/nginx.conf \
		$(DEPLOY_USER)@$(DEPLOY_HOST):/etc/nginx/sites-available/grav-charge-sim
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) " \
		mkdir -p $(DEPLOY_DIR) && \
		ln -sf /etc/nginx/sites-available/grav-charge-sim \
		        /etc/nginx/sites-enabled/grav-charge-sim && \
		rm -f /etc/nginx/sites-enabled/default && \
		nginx -t && systemctl reload nginx && \
		systemctl daemon-reload && \
		systemctl enable grav-charge-sim"
	@echo "Server ready. Run 'make deploy' to push the first binary."

# ----- clean -----------------------------------------------------------

clean:
	rm -f client/sim.wasm client/wasm_exec.js $(BINARY)
