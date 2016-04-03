# MySQL Server Exporter [![Build Status](https://travis-ci.org/prometheus/mysqld_exporter.svg)](https://travis-ci.org/prometheus/mysqld_exporter)

Prometheus exporter for MySQL server metrics.
Supported MySQL versions: 5.1 and up.
NOTE: Not all collection methods are support on MySQL < 5.6

## Building and running

### Required Grants

    CREATE USER 'exporter'@'localhost' IDENTIFIED BY 'XXXXXXXX';
    GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'exporter'@'localhost';
    GRANT SELECT ON performance_schema.* TO 'exporter'@'localhost';

### Build

    make

### Running

Running using an enviornment variable:

    export DATA_SOURCE_NAME='login:password@(hostname:port)/'
    ./mysqld_exporter <flags>

Running using ~/.my.cnf:

    ./mysqld_exporter <flags>

### Collector Flags

Name                                                   | MySQL Version | Description
-------------------------------------------------------|---------------|------------------------------------------------------------------------------------
collect.global_status                                  | 5.1           | Collect from SHOW GLOBAL STATUS (Enabled by default)
collect.global_variables                               | 5.1           | Collect from SHOW GLOBAL VARIABLES (Enabled by default)
collect.slave_status                                   | 5.1           | Collect from SHOW SLAVE STATUS (Enabled by default)
collect.binlog_size                                    | 5.1           | Collect the current size of all registered binlog files
collect.info_schema.innodb_metrics                     | 5.6           | Collect metrics from information_schema.innodb_metrics.
collect.auto_increment.columns                         | 5.1           | Collect auto_increment columns and max values from information_schema.
collect.engine_tokudb_status                           | 5.6           | Collect from SHOW ENGINE TOKUDB STATUS.
collect.info_schema.userstats                          | 5.1           | If running with userstat=1, set to true to collect user statistics.
collect.info_schema.tablestats                         | 5.1           | If running with userstat=1, set to true to collect table statistics.
collect.info_schema.tables                             | 5.1           | Collect metrics from information_schema.tables.
collect.info_schema.tables.databases                   | 5.1           | The list of databases to collect table stats for, or '`*`' for all.
collect.info_schema.query_response_time                | 5.5           | Collect query response time distribution if query_response_time_stats is ON.
collect.info_schema.processlist                        | 5.1           | Collect thread state counts from information_schema.processlist.
collect.info_schema.processlist.min_time               | 5.1           | Minimum time a thread must be in each state to be counted
collect.perf_schema.eventsstatements                   | 5.6           | Collect metrics from performance_schema.events_statements_summary_by_digest.
collect.perf_schema.eventsstatements.limit             | 5.6           | Limit the number of events statements digests by response time. (default: 250)
collect.perf_schema.eventsstatements.digest_text_limit | 5.6           | Maximum length of the normalized statement text. (default: 120)
collect.perf_schema.indexiowaits                       | 5.6           | Collect metrics from performance_schema.table_io_waits_summary_by_index_usage.
collect.perf_schema.tableiowaits                       | 5.6           | Collect metrics from performance_schema.table_io_waits_summary_by_table.
collect.perf_schema.tablelocks                         | 5.6           | Collect metrics from performance_schema.table_lock_waits_summary_by_table.
collect.perf_schema.file_events                        | 5.5           | Collect metrics from performance_schema.file_summary_by_event_name.
collect.perf_schema.eventswaits                        | 5.5           | Collect metrics from performance_schema.events_waits_summary_global_by_event_name.


### General Flags
Name                                       | Description
-------------------------------------------|--------------------------------------------------------------------------------------------------
config.my-cnf                              | Path to .my.cnf file to read MySQL credentials from. (default: `~/.my.cnf`)
log.level                                  | Logging verbosity (default: info)
log_slow_filter                            | Add a log_slow_filter to avoid exessive MySQL slow logging.  NOTE: Not supported by Oracle MySQL.
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
