.PHONY: all build clean test server itest grpc-echo multimodal extended-card

all: build

build: server itest grpc-echo multimodal extended-card

server:
	go build -o bin/server ./cmd/server

itest:
	go build -o bin/itest ./cmd/itest

grpc-echo:
	go build -o bin/grpc-echo ./cmd/grpc-echo

multimodal:
	go build -o bin/multimodal ./cmd/multimodal

extended-card:
	go build -o bin/extended-card ./cmd/extended-card

test:
	go test ./...

clean:
	rm -rf bin/
