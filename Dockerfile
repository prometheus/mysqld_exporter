
FROM alpine:3.17 as builder
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

WORKDIR /usr/src/mysqld_exporter

COPY . .

RUN apk add --no-cache \
    git make musl-dev go; \
    go build -v


FROM alpine:3.17 as app

COPY --from=builder /usr/src/mysqld_exporter/mysqld_exporter /bin/mysqld_exporter
EXPOSE 9104

CMD  [ "/bin/mysqld_exporter" ]
