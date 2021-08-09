ADDR=localhost:2200
LDFLAGS = -s -w

ifeq "$(GOOS)" "windows"
	LDFLAGS += -H=windowsgui
endif

release: .generate_keys
	go build -ldflags="$(LDFLAGS)" -o bin ./...

debug: .generate_keys
	go build  -o bin ./...

client: .generate_keys
	go build -ldflags="$(LDFLAGS)" -o bin ./cmd/client

run: 
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	cd bin; ./server $(ADDR)

.generate_keys:
	mkdir -p bin
	ssh-keygen -t ed25519 -N '' -f internal/client/keys/private_key; 
	cat internal/client/keys/private_key.pub >> bin/authorized_controllee_keys;

	

	
