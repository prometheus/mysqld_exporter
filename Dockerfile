FROM        quay.io/prometheus/busybox:latest
MAINTAINER  The Prometheus Authors <prometheus-developers@googlegroups.com>

COPY mysqld_exporter /bin/mysqld_exporter

EXPOSE      9104
ENTRYPOINT  [ "/bin/mysqld_exporter" ]
