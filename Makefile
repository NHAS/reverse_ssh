ADDR=localhost:2200


debug: 
	mkdir -p bin
	go build -o bin ./...

release:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin ./...

run: 
	./bin/client $(ADDR)
	./bin/client $(ADDR)
	./bin/client $(ADDR)
	cd bin; ./server $(ADDR)
	

	