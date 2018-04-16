# MySQL Server Exporter [![Build Status](https://travis-ci.org/prometheus/mysqld_exporter.svg)][travis]

[![CircleCI](https://circleci.com/gh/prometheus/mysqld_exporter/tree/master.svg?style=shield)][circleci]
[![Docker Repository on Quay](https://quay.io/repository/prometheus/mysqld-exporter/status)][quay]
[![Docker Pulls](https://img.shields.io/docker/pulls/prom/mysqld-exporter.svg?maxAge=604800)][hub]
[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/mysqld_exporter)](https://goreportcard.com/report/github.com/prometheus/mysqld_exporter)

Prometheus exporter for MySQL server metrics.
Supported MySQL versions: 5.1 and up.
NOTE: Not all collection methods are supported on MySQL < 5.6

## Building and running

### Required Grants

```sql
CREATE USER 'exporter'@'localhost' IDENTIFIED BY 'XXXXXXXX' WITH MAX_USER_CONNECTIONS 3;
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'localhost';
```

NOTE: It is recommended to set a max connection limit for the user to avoid overloading the server with monitoring scrapes under heavy load.

### Build

    make

### Running

Running using an environment variable:

    export DATA_SOURCE_NAME='login:password@(hostname:port)/'
    ./mysqld_exporter <flags>

Running using ~/.my.cnf:

    ./mysqld_exporter <flags>

Example format for flags for version > 0.10.0:
  
    --collect.auto_increment.columns
    --no-collect.auto_increment.columns
  
Example format for flags for version <= 0.10.0:
  
    -collect.auto_increment.columns
    -collect.auto_increment.columns=[true|false]

### Collector Flags

Name                                                   | MySQL Version | Description
-------------------------------------------------------|---------------|------------------------------------------------------------------------------------
collect.auto_increment.columns                         | 5.1           | Collect auto_increment columns and max values from information_schema.
collect.binlog_size                                    | 5.1           | Collect the current size of all registered binlog files
collect.engine_innodb_status                           | 5.1           | Collect from SHOW ENGINE INNODB STATUS.
collect.engine_tokudb_status                           | 5.6           | Collect from SHOW ENGINE TOKUDB STATUS.
collect.global_status                                  | 5.1           | Collect from SHOW GLOBAL STATUS (Enabled by default)
collect.global_variables                               | 5.1           | Collect from SHOW GLOBAL VARIABLES (Enabled by default)
collect.info_schema.clientstats                        | 5.5           | If running with userstat=1, set to true to collect client statistics.
collect.info_schema.innodb_metrics                     | 5.6           | Collect metrics from information_schema.innodb_metrics.
collect.info_schema.innodb_tablespaces                 | 5.7           | Collect metrics from information_schema.innodb_sys_tablespaces.
collect.info_schema.innodb_cmp                	       | 5.5           | Collect metrics from information_schema.innodb_cmp.
collect.info_schema.innodb_cmpmem              	       | 5.5           | Collect metrics from information_schema.innodb_cmpmem.
collect.info_schema.processlist                        | 5.1           | Collect thread state counts from information_schema.processlist.
collect.info_schema.processlist.min_time               | 5.1           | Minimum time a thread must be in each state to be counted. (default: 0)
collect.info_schema.query_response_time                | 5.5           | Collect query response time distribution if query_response_time_stats is ON.
collect.info_schema.tables                             | 5.1           | Collect metrics from information_schema.tables (Enabled by default)
collect.info_schema.tables.databases                   | 5.1           | The list of databases to collect table stats for, or '`*`' for all.
collect.info_schema.tablestats                         | 5.1           | If running with userstat=1, set to true to collect table statistics.
collect.info_schema.userstats                          | 5.1           | If running with userstat=1, set to true to collect user statistics.
collect.perf_schema.eventsstatements                   | 5.6           | Collect metrics from performance_schema.events_statements_summary_by_digest.
collect.perf_schema.eventsstatements.digest_text_limit | 5.6           | Maximum length of the normalized statement text. (default: 120)
collect.perf_schema.eventsstatements.limit             | 5.6           | Limit the number of events statements digests by response time. (default: 250)
collect.perf_schema.eventsstatements.timelimit         | 5.6           | Limit how old the 'last_seen' events statements can be, in seconds. (default: 86400)
collect.perf_schema.eventswaits                        | 5.5           | Collect metrics from performance_schema.events_waits_summary_global_by_event_name.
collect.perf_schema.file_events                        | 5.6           | Collect metrics from performance_schema.file_summary_by_event_name.
collect.perf_schema.file_instances                     | 5.5           | Collect metrics from performance_schema.file_summary_by_instance.
collect.perf_schema.indexiowaits                       | 5.6           | Collect metrics from performance_schema.table_io_waits_summary_by_index_usage.
collect.perf_schema.tableiowaits                       | 5.6           | Collect metrics from performance_schema.table_io_waits_summary_by_table.
collect.perf_schema.tablelocks                         | 5.6           | Collect metrics from performance_schema.table_lock_waits_summary_by_table.
collect.slave_status                                   | 5.1           | Collect from SHOW SLAVE STATUS (Enabled by default)
collect.heartbeat                                      | 5.1           | Collect from [heartbeat](#heartbeat).
collect.heartbeat.database                             | 5.1           | Database from where to collect heartbeat data. (default: heartbeat)
collect.heartbeat.table                                | 5.1           | Table from where to collect heartbeat data. (default: heartbeat)


### General Flags
Name                                       | Description
-------------------------------------------|--------------------------------------------------------------------------------------------------
config.my-cnf                              | Path to .my.cnf file to read MySQL credentials from. (default: `~/.my.cnf`)
log.level                                  | Logging verbosity (default: info)
exporter.lock_wait_timeout                 | Set a lock_wait_timeout on the connection to avoid long metadata locking. (default: 2 seconds)
exporter.log_slow_filter                   | Add a log_slow_filter to avoid slow query logging of scrapes.  NOTE: Not supported by Oracle MySQL.
web.listen-address                         | Address to listen on for web interface and telemetry.
web.telemetry-path                         | Path under which to expose metrics.
version                                    | Print the version information.

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

## heartbeat

With `collect.heartbeat` enabled, mysqld_exporter will scrape replication delay
measured by heartbeat mechanisms. [Pt-heartbeat][pth] is the
reference heartbeat implementation supported.

[pth]:https://www.percona.com/doc/percona-toolkit/2.2/pt-heartbeat.html


## Prometheus Configuration

The mysqld exporter will expose all metrics from enabled collectors by default, but it can be passed an optional list of collectors to filter metrics. The `collect[]` parameter accepts values matching [Collector Flags](#collector-flags) names (without `collect.` prefix).

This can be useful for specifying different scrape intervals for different collectors.

```yaml
scrape_configs:
  - job_name: 'mysql global status'
    scrape_interval: 15s
    static_configs:
      - targets:
        - '192.168.1.2:9104'
    params:
      collect[]:
        - global_status

  - job_name: 'mysql performance'
    scrape_interval: 1m
    static_configs:
      - targets:
        - '192.168.1.2:9104'
    params:
      collect[]:
        - perf_schema.tableiowaits
        - perf_schema.indexiowaits
        - perf_schema.tablelocks
```

## Example Rules

There are some sample rules available in [example.rules](example.rules)

[circleci]: https://circleci.com/gh/prometheus/mysqld_exporter
[hub]: https://hub.docker.com/r/prom/mysqld-exporter/
[travis]: https://travis-ci.org/prometheus/mysqld_exporter
[quay]: https://quay.io/repository/prometheus/mysqld-exporter
