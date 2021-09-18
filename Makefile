ADDR=localhost:2200
LDFLAGS = -s -w

ifeq "$(GOOS)" "windows"
	LDFLAGS += -H=windowsgui
endif

debug: .generate_keys
	go build  -o bin ./...

release: .generate_keys
	go build -ldflags="$(LDFLAGS)" -o bin ./...

client: .generate_keys
	go build -ldflags="$(LDFLAGS)" -o bin ./cmd/client

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
	touch bin/authorized_controllee_keys
	@grep -q "$$(cat internal/client/keys/private_key.pub)" bin/authorized_controllee_keys || cat internal/client/keys/private_key.pub >> bin/authorized_controllee_keys
