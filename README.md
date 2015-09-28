# MySQL Server Exporter

Prometheus exporter for MySQL server metrics.
Supported MySQL versions: 5.1 and up.
NOTE: Not all collection methods are support on MySQL < 5.6

## Building and running

    make
    export DATA_SOURCE_NAME="login:password@(hostname:port)/dbname"
    ./mysqld_exporter <flags>

### Flags

Name                                       | Description
-------------------------------------------|------------------------------------------------------------------------------------
collect.auto_increment.columns             | Collect auto_increment columns and max values from information_schema.
collect.binlog_size                        | Compute the size of all binlog files combined (as specified by "SHOW MASTER LOGS")
collect.info_schema.userstats              | If running with userstat=1, set to true to collect user statistics.
collect.perf_schema.eventsstatements       | Collect time metrics from performance_schema.events_statements_summary_by_digest.
collect.perf_schema.eventsstatements.limit | Limit the number of events statements digests by response time. (default: 250)
collect.perf_schema.eventsstatements.digest_text_limit | Maximum length of the normalized statement text. (default: 120)
collect.perf_schema.indexiowaits           | Collect metrics from performance_schema.table_io_waits_summary_by_index_usage.
collect.perf_schema.indexiowaitstime       | Collect time metrics from performance_schema.table_io_waits_summary_by_index_usage.
collect.perf_schema.tableiowaits           | Collect metrics from performance_schema.table_io_waits_summary_by_table.
collect.perf_schema.tableiowaitstime       | Collect time metrics from performance_schema.table_io_waits_summary_by_table.
collect.perf_schema.tablelocks             | Collect metrics from performance_schema.table_lock_waits_summary_by_table.
collect.perf_schema.tablelockstime         | Collect time metrics from performance_schema.events_statements_summary_by_digest.
log.level                                  | Logging verbosity (default: info)
web.listen-address                         | Address to listen on for web interface and telemetry.
web.telemetry-path                         | Path under which to expose metrics.

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

## Example queries

Certain queries to `events_statements` metrics will contain a query digest
label in the result vector. The `events_statements_digest_text` metrics
stores a mapping of those digests to their actual query template (digest_text).
This metric can be used to join in the digest text to a query result so it can
be interpreted in a more meaningful way.

Suppose a query is:

    sum(rate(mysql_perf_schema_events_statements_total[5m])) by (digest)

The result vector contains the rate of events per digest, which is of little
meaning without knowing the actual query behind the digest.

Extending the query adds a `digest_text` label to the result vector showing us
exactly that information:

    (sum(rate(mysql_perf_schema_events_statements_total[5m])) by (digest)) * on(digest) group_right(schema, digest_text) mysql_perf_schema_events_statements_digest_text

This extension (`* on(digest)...`) is generally applicable to all similar
queries.

