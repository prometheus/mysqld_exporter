# mysqld_exporter

Exporter for MySQL server metrics http://prometheus.io/

## Building and running

    make
    ./mysqld_exporter <flags>

Flags description

Name               | Description
-------------------|------------
web.listen-address | Address to listen on for web interface and telemetry.
web.telemetry-path | Path under which to expose metrics.
config.file        | Config file name with connection string

## Configuration
The configuration is in JSON. An example with all possible options:
```
{
   "config" : {
      "mysql_connection" : "login:password@host/dbname"
   }
}
```

Format of connection string described at https://github.com/go-sql-driver/mysql#dsn-data-source-name
