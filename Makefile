ADDR=localhost:2200

ifeq "$(GOOS)" "windows"
	LDFLAGS += -H=windowsgui
endif

ifdef RSSH_HOMESERVER
	LDFLAGS += -X main.defaultClientDestination=$(RSSH_HOMESERVER)
endif

LDFLAGS_RELEASE = $(LDFLAGS) -s -w

debug: .generate_keys
	go build -ldflags="$(LDFLAGS)" -o bin ./...

release: .generate_keys
	go build -ldflags="$(LDFLAGS_RELEASE)" -o bin ./...

client: .generate_keys
	go build -ldflags="$(LDFLAGS_RELEASE)" -o bin ./cmd/client

run:
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	cd bin; ./server $(ADDR)

.generate_keys:
	mkdir -p bin
# Supress errors if user doesn't overwrite existing key
	ssh-keygen -t ed25519 -N '' -f internal/client/keys/private_key || true
# Avoid duplicate entries
	@grep -q "$$(cat internal/client/keys/private_key.pub)" bin/authorized_controllee_keys || cat internal/client/keys/private_key.pub >> bin/authorized_controllee_keys
