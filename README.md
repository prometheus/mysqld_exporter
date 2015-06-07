# MySQL Server Exporter

Prometheus exporter for MySQL server metrics.
Supported MySQL versions: 5.1 and up.

## Building and running

    make
    export DATA_SOURCE_NAME="login:password@(hostname:port)/dbname"
    ./mysqld_exporter <flags>

### Flags

Name               | Description
-------------------|------------
web.listen-address | Address to listen on for web interface and telemetry.
web.telemetry-path | Path under which to expose metrics.

### Setting the MySQL server's data source name

The MySQL server's [data source name](http://en.wikipedia.org/wiki/Data_source_name)
must be set via the `DATA_SOURCE_NAME` environment variable.
The format of this variable is described at https://github.com/go-sql-driver/mysql#dsn-data-source-name.

## Using Docker

You can deploy this exporter using the [prom/mysqld-exporter](https://registry.hub.docker.com/u/prom/mysqld-exporter/) Docker image.

For example:

```bash
docker pull prom/mysqld-exporter

docker run -d -p 9104:9104 --link=my_mysql_container:bdd  \
        -e DATA_SOURCE_NAME="user:password@(bdd:3306)/database" prom/mysqld-exporter
```
