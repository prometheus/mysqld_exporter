# mysqld_exporter

Exporter for MySQL server metrics http://prometheus.io/
Supported mysql version 5.1 and up

## Building and running

    make
    export DATA_SOURCE_NAME="login:password@host/dbname"
    ./mysqld_exporter <flags>

Flags description

Name               | Description
-------------------|------------
web.listen-address | Address to listen on for web interface and telemetry.
web.telemetry-path | Path under which to expose metrics.

Variable with database datasource must be set in DATA_SOURCE_NAME environment variable
Format of connection string described at https://github.com/go-sql-driver/mysql#dsn-data-source-name
