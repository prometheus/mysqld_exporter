ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

LABEL org.opencontainers.image.authors="The Prometheus Authors"
LABEL org.opencontainers.image.vendor="Prometheus"
LABEL org.opencontainers.image.title="mysqld_exporter"
LABEL org.opencontainers.image.description="Prometheus exporter for MySQL server metrics"
LABEL org.opencontainers.image.source="https://github.com/prometheus/mysqld_exporter"
LABEL org.opencontainers.image.url="https://github.com/prometheus/mysqld_exporter"
LABEL org.opencontainers.image.documentation="https://github.com/prometheus/mysqld_exporter/blob/main/README.md"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL io.prometheus.image.variant="busybox"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/mysqld_exporter /bin/mysqld_exporter

EXPOSE      9104
USER        nobody
ENTRYPOINT  [ "/bin/mysqld_exporter" ]
