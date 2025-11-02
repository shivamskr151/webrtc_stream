.PHONY: install run-signaling run-publisher build clean

install:
	go mod download

run-signaling:
	go run cmd/signaling/main.go

run-publisher:
	go run cmd/publisher/main.go

build:
	go build -o bin/signaling cmd/signaling/main.go
	go build -o bin/publisher cmd/publisher/main.go

clean:
	rm -rf bin/

