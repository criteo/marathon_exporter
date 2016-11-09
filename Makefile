VERSION  := 0.3.2
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
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build \
		-a -tags netgo \
		-ldflags "-X main.Version=$(VERSION)" \
		-o bin/$(TARGET) .

publish: release
	docker build -t gettyimages/$(TARGET):$(VERSION) .
	docker push gettyimages/$(TARGET):$(VERSION)
	docker tag gettyimages/$(TARGET):$(VERSION) gettyimages/$(TARGET):latest
	docker push gettyimages/$(TARGET):latest

clean:
	rm -rf bin/
