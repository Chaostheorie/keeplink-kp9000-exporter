.PHONY: build run test docker clean

build:
	go build -o bin/exporter ./cmd/exporter

run:
	./bin/exporter -i 192.168.1.168

test:
	go test ./...

docker:
	docker build -t sergiosanche/keeplink-kp9000-exporter .

clean:
	rm -rf bin/

