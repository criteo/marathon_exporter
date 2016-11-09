# Marathon Prometheus Exporter

[![Build Status](https://travis-ci.org/gettyimages/marathon_exporter.svg?branch=master)](https://travis-ci.org/gettyimages/marathon_exporter)

A [Prometheus](http://prometheus.io) metrics exporter for the [Marathon](https://mesosphere.github.io/marathon) Mesos framework.

This exporter exposes Marathon's Codahale/Dropwizard metrics via its `/metrics` endpoint. To learn more, visit the [Marathon metrics doc](http://mesosphere.github.io/marathon/docs/metrics.html).

## Getting

```sh
$ go get github.com/gettyimages/marathon_exporter
```

*\-or-*

```sh
$ docker pull gettyimages/marathon_exporter
```

*\-or-*

```
make deps && make
bin/marathon_exporter --help
```

## Using

```sh
Usage of marathon_exporter:
  -marathon.uri string
        URI of Marathon (default "http://marathon.mesos:8080")
  -web.listen-address string
        Address to listen on for web interface and telemetry. (default ":9088")
  -web.telemetry-path string
        Path under which to expose metrics. (default "/metrics")
  -log.format value
        If set use a syslog logger or JSON logging. Example: logger:syslog?appname=bob&local=7 or logger:stdout?json=true. Defaults to stderr.
  -log.level value
        Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]. (default info)
```
