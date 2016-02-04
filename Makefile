VERSION  := 0.1.0
TARGET   := marathon_exporter
TEST     ?= ./...

default: test build

test:
	go test -v $(TEST)

build: clean
	go build -o bin/$(TARGET)

clean:
	rm -rf bin/
