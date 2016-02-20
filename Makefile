VERSION  := 0.2.1
TARGET   := marathon_exporter
TEST     ?= ./...

default: test build

deps:
	go get -v -u ./...

test:
	go test -v -run=$(RUN) $(TEST)

build: clean
	go build -v -o bin/$(TARGET)

release: clean
	GOARCH=amd64 GOOS=linux go build -ldflags "-X main.Version=$(VERSION)" -o bin/$(TARGET) .

publish: release
	docker build -t gettyimages/$(TARGET):$(VERSION) .
	docker push gettyimages/$(TARGET):$(VERSION)

clean:
	rm -rf bin/
