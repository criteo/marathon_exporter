FROM scratch
MAINTAINER Getty Images "https://github.com/gettyimages"

ADD bin/marathon_exporter /marathon_exporter
ENTRYPOINT ["/marathon_exporter"]

EXPOSE 9088
