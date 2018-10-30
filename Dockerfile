FROM golang:1.9-alpine3.7 as builder
RUN apk add --update \
    make \
    git \
  && rm -rf /var/cache/apk/*
RUN mkdir -p /go/src/github.com/gettyimages/marathon_exporter
ADD . /go/src/github.com/gettyimages/marathon_exporter
WORKDIR /go/src/github.com/gettyimages/marathon_exporter
RUN make build

FROM alpine:3.7

COPY --from=builder /go/src/github.com/gettyimages/marathon_exporter/bin/marathon_exporter /marathon_exporter
ENTRYPOINT ["/marathon_exporter"]

EXPOSE 9088
