FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Xavier MARCELET <xavier.marcelet@orange.com>

COPY credhub_exporter /bin/credhub_exporter

ENTRYPOINT ["/bin/credhub_exporter"]
EXPOSE     9358
