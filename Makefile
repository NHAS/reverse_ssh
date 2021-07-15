ADDR=localhost:2200

debug: 
	mkdir -p bin
	ssh-keygen -t ed25519 -N '' -f internal/client/keys/private_key
	cat internal/client/keys/private_key.pub >> bin/authorized_controllee_keys
	go build -o bin ./...

release:
	mkdir -p bin
	ssh-keygen -t ed25519 -N '' -f internal/client/keys/private_key
	cat internal/client/keys/private_key.pub >> bin/authorized_controllee_keys
	go build -ldflags="-s -w" -o bin ./...

client:
	go build -ldflags="-s -w" -o bin ./cmd/client

run: 
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	./bin/client --reconnect $(ADDR)
	cd bin; ./server $(ADDR)
	

	
