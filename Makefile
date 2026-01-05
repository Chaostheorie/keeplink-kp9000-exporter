.PHONY: build run test docker clean

build:
	go build -o bin/exporter-keeplink ./cmd/exporter

run:
	./bin/exporter-keeplink -i 192.168.1.168 --cookie="admin=f6fdffe48c908deb0f4c3bd36c032e72" --listen=:9101

test:
	go test ./...

docker:
	docker build -t sergiosanche/keeplink-kp9000-exporter .

clean:
	rm -rf bin/

