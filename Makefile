VERSION  := 0.1.0
TARGET   := marathon_exporter
TEST     ?= ./...

default: test build

test:
	go test -v -run=$(RUN) $(TEST)

build: clean
	go build -o bin/$(TARGET)

release: clean
	GOARCH=amd64 GOOS=linux go build -ldflags "-X main.Version=$(VERSION)" -o bin/$(TARGET) .

publish: release
	docker build -t gettyimages/$(TARGET):$(VERSION) .
	docker push gettyimages/$(TARGET):$(VERSION)

clean:
	rm -rf bin/
