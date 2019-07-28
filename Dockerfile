ARG ARCH="amd64"
ARG OS="linux"
FROM   quay.io/prometheus/busybox:latest
LABEL  maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/mysqld_exporter /bin/mysqld_exporter

USER        nobody
EXPOSE      9104
ENTRYPOINT  [ "/bin/mysqld_exporter" ]
