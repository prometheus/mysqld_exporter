## master / unreleased

BREAKING CHANGES:

Changes:

* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]

## 0.14.0 / 2022-01-05

BREAKING CHANGES:

Metric names in the info_schema.processlist collector have been changed. #603
Metric names in the info_schema.replica_host collector have been changed. #496

* [CHANGE] Rewrite processlist collector #603
* [FEATURE] Add collector for `replica_host_status` #496
* [ENHANCEMENT] Expose dates as timestamps grom GLOBAL STATUS #561
* [BUGFIX] Fix mysql_slave_hosts_info for mysql 5.5 and mariadb 10.5 #577
* [BUGFIX] Fix logging issues #562 #602

## 0.13.0 / 2021-05-18

BREAKING CHANGES:

Changes related to `replication_group_member_stats` collector:
* metric "transaction_in_queue" was Counter instead of Gauge
* renamed 3 metrics starting with `mysql_perf_schema_transaction_` to start with `mysql_perf_schema_transactions_` to be consistent with column names
* exposing only server's own stats by matching MEMBER_ID with @@server_uuid resulting "member_id" label to be dropped.

Changes:

* [CHANGE] Switch to go-kit for logs. #433
* [FEATURE] Add `tls.insecure-skip-verify` flag to ignore tls verification errors #417
* [FEATURE] Add collector for AWS Aurora information_schema.replica_host_status #435
* [FEATURE] Add collector for `replication_group_members` #459
* [FEATURE] Add new metrics to `replication_group_member_stats` collector to support MySQL 8.x. #462
* [FEATURE] Add collector for `performance_schema.memory_summary_global_by_event_name` #515
* [FEATURE] Support authenticating using mTLS client cert and no password #539
* [FEATURE] Add TLS and basic authentication #522
* [ENHANCEMENT] Support heartbeats in UTC #471
* [ENHANCEMENT] Improve parsing of boolean strings #548
* [BUGFIX] Fix binlog metrics on mysql 8.x #419
* [BUGFIX] Fix output value of wsrep_cluster_status #473
* [BUGFIX] Fix collect.info_schema.innodb_metrics for new field names (mariadb 10.5+) #494
* [BUGFIX] Fix log output of collect[] params #505
* [BUGFIX] Fix collect.info_schema.innodb_tablespaces for new table names #516
* [BUGFIX] Fix innodb_metrics for mariadb 10.5+ #523
* [BUGFIX] Allow perf_schema.memory summary current_bytes to be negative #517


## 0.12.1 / 2019-07-10

### Changes:

* Rebuild to update Docker packages.

## 0.12.0 / 2019-07-10

### BREAKING CHANGES:

The minimum supported MySQL version is now 5.5.

Collector `info_schema.tables` is now disabled by default due to high cardinality danger.

### Changes:

* [CHANGE] Update defaults for MySQL 5.5 #318
* [CHANGE] Update innodb buffer pool mappings #369
* [CHANGE] Disable info_schema.tables collector by default #406
* [BUGFIX] Sanitize metric names in global variables #307
* [BUGFIX] Use GLOBAL to prevent mysql deadlock #336
* [BUGFIX] Clear last_scrape_error on every scrape (PR #368) #367
* [ENHANCEMENT] Add help for some GLOBAL VARIABLES metrics. #326
* [FEATURE] Abort on timeout. #323
* [FEATURE] Add minimal MySQL version to Scraper interface #328
* [FEATURE] Add by_user and by_host metrics to info_schema.processlist collector (PR #333) #334
* [FEATURE] Add wsrep_evs_repl_latency metric collecting. (PR #338)
* [FEATURE] Add collector for mysql.user (PR #341)
* [FEATURE] Add perf_schema.eventsstatementssum collector #347
* [FEATURE] Add collector to get table stats grouped by schema (PR #354)
* [FEATURE] Add replication_applier_status_by_worker metric collecting. (PR #366)

## 0.11.0 / 2018-06-29

### BREAKING CHANGES:
* Flags now use the Kingpin library, and require double-dashes. #222

This also changes the behavior of boolean flags.
* Enable: `--collect.global_status`
* Disable: `--no-collect.global_status`

### Changes:
* [CHANGE] Limit number and lifetime of connections #208
* [ENHANCEMENT] Move session params to DSN #259
* [ENHANCEMENT] Use native DB.Ping() instead of self-written implementation #210
* [FEATURE] Add collector duration metrics #197
* [FEATURE] Add 'collect[]' URL parameter to filter enabled collectors #235
* [FEATURE] Set a `lock_wait_timeout` on the MySQL connection #252
* [FEATURE] Set `last_scrape_error` when an error occurs #237
* [FEATURE] Collect metrics from `performance_schema.replication_group_member_stats` #271
* [FEATURE] Add innodb compression statistic #275
* [FEATURE] Add metrics for the output of `SHOW SLAVE HOSTS` #279
* [FEATURE] Support custom CA truststore and client SSL keypair. #255
* [BUGFIX] Fix perfEventsStatementsQuery #213
* [BUGFIX] Fix `file_instances` metric collector #205
* [BUGFIX] Fix prefix removal in `perf_schema_file_instances` #257
* [BUGFIX] Fix 32bit compile issue #273
* [BUGFIX] Ignore boolean keys in my.cnf. #283

## 0.10.0 / 2017-04-25

### BREAKING CHANGES:
* `mysql_slave_...` metrics now include an additional `connection_name` label to support mariadb multi-source replication. (#178)

### Changes:
* [FEATURE] Add read/write query response time #166
* [FEATURE] Add Galera gcache size metric #169
* [FEATURE] Add MariaDB multi source replication support #178
* [FEATURE] Implement heartbeat metrics #183
* [FEATURE] Add basic `file_summary_by_instance` metrics #189
* [BUGFIX] Workaround MySQL bug 79533 #173

## 0.9.0 / 2016-09-26

### BREAKING CHANGES:
* InnoDB buffer pool page stats have been renamed/fixed to better support aggregations (#130)

### Changes:
* [FEATURE] scrape slave status for multisource replication #134
* [FEATURE] Add client statistics support (+ add tests on users & clients statistics) #138
* [IMPROVEMENT] Consistency of error logging. #144
* [IMPROVEMENT] Add label aggregation for innodb buffer metrics #130
* [IMPROVEMENT] Improved and fixed user/client statistics #149
* [FEATURE] Added the last binlog file number metric. #152
* [MISC] Add an example recording rules file #156
* [FEATURE] Added PXC/Galera info metrics. #155
* [FEATURE] Added metrics from SHOW ENGINE INNODB STATUS. #160
* [IMPROVEMENT] Fix `wsrep_cluster_status` #146


## 0.8.1 / 2016-05-05

### Changes:
* [BUGFIX] Fix `collect.info_schema.innodb_tablespaces` #119
* [BUGFIX] Fix SLAVE STATUS "Connecting" #125
* [MISC] New release process using docker, circleci and a centralized building tool #120
* [MISC] Typos #121

## 0.8.0 / 2016-04-19

### BREAKING CHANGES:
* global status `innodb_buffer_pool_pages` have been renamed/labeled.
* innodb metrics `buffer_page_io` have been renamed/labeled.

### Changes:
* [MISC] Add Travis CI automatic testing.
* [MISC] Refactor `mysqld_exporter.go` into collector package.
* [FEATURE] Add `mysql_up` metric (PR #99)
* [FEATURE] Collect time metrics for processlist (PR #87)
* [CHANGE] Separate `innodb_buffer_pool_pages` status metrics (PR #101)
* [FEATURE] Added metrics from SHOW ENGINE TOKUDB STATUS (PR #103)
* [CHANGE] Add special handling of `buffer_page_io` subsystem (PR #115)
* [FEATURE] Add collector for `innodb_sys_tablespaces` (PR #116)

## 0.7.1 / 2016-02-16

### Changes:
* [IMPROVEMENT] Soft error on collector failure (PR #84)
* [BUGFIX] Fix `innodb_metrics` collector (PR #85)
* [BUGFIX] Parse auto increment values and maximum as float64 (PR #88)

## 0.7.0 / 2016-02-12

### BREAKING CHANGES:
* Global status metrics for "handlers" have been renamed

### Changes:
* [FEATURE] New collector for `information_schema.table_statistics` (PR #57)
* [FEATURE] New server version metric (PR #59)
* [FEATURE] New collector for `information_schema.innodb_metrics` (PR #69)
* [FEATURE] Read credentials from ".my.cnf" files (PR #77)
* [FEATURE] New collector for query response time distribution (PR #79)
* [FEATURE] Add minimum time flag for processlist metrics (PR #82)
* [IMPROVEMENT] Collect more metrics from `performance_schema.events_statements_summary_by_digest` (PR #58)
* [IMPROVEMENT] Add option to filter metrics queries from the slow log (PR #60)
* [IMPROVEMENT] Leverage lock-free SHOW SLAVE STATUS (PR #61)
* [IMPROVEMENT] Add labels to global status "handlers" counters (PR #68)
* [IMPROVEMENT] Update Makefile.COMMON from utils repo (PR #73)
* [BUGFIX] Fix broken error return in the scrape function and log an error (PR #64)
* [BUGFIX] Check `log_bin` before running SHOW BINARY LOGS (PR #74)
* [BUGFIX] Fixed uint for scrapeInnodbMetrics() and gofmt (PR #81)

## 0.6.0 / 2015-10-28

### BREAKING CHANGES:
* The `digest_text` mapping metric has been removed, now included in all digest metrics (PR #50)
* Flags for timing metrics have been removed, now included with related counter flag (PR #48)

### Changes:
* [FEATURE] New collector for metrics from information_schema.processlist (PR #34)
* [FEATURE] New collector for binlog counts/sizes (PR #35)
* [FEATURE] New collector for `performance_schema.{file_summary_by_event_name,events_waits_summary_global_by_event_name}` (PR #49)
* [FEATURE] New collector for `information_schema.tables` (PR #51)
* [IMPROVEMENT] All collection methods now have enable flags (PR #46)
* [IMPROVEMENT] Consolidate `performance_schema` metrics flags (PR #48)
* [IMPROVEMENT] Removed need for `digest_text` mapping metric (PR #50)
* [IMPROVEMENT] Update docs (PR #52)

## 0.5.0 / 2015-09-22

### Changes:
* [FEATURE] Add metrics for table locks
* [BUGFIX] Use uint64 to prevent int64 overflow
* [BUGFIX] Correct picsecond times to correct second values

## 0.4.0 / 2015-09-21

### Changes:
* [CHANGE] Limit `events_statements` to recently used
* [FEATURE] Add `digest_text` mapping metric
* [IMPROVEMENT] General refactoring

## 0.3.0 / 2015-08-31

### BREAKING CHANGES:
* Most metrics have been prefixed with Prometheus subsystem names to avoid conflicts between different collection methods.

### Changes:
* [BUGFIX] Separate `slave_status` and `global_status` into separate subsystems.
* [IMPROVEMENT] Refactor metrics creation.
* [IMPROVEMENT] Add support for `performance_schema.table_io_waits_summary_by_table` collection.
* [IMPROVEMENT] Add support for `performance_schema.table_io_waits_summary_by_index_usage` collection.
* [IMPROVEMENT] Add support for `performance_schema.events_statements_summary_by_digest` collection.
* [IMPROVEMENT] Add support for Percona userstats output collection.
* [IMPROVEMENT] Add support for `auto_increment` column metrics collection.
* [IMPROVEMENT] Add support for `SHOW GLOBAL VARIABLES` metrics collection.

## 0.2.0 / 2015-06-24

### BREAKING CHANGES:
* Logging-related flags have changed. Metric names have changed.

### Changes:
* [IMPROVEMENT] Add Docker support.
* [CHANGE] Switch logging to Prometheus' logging library.
* [BUGFIX] Fix slave status parsing.
* [BUGFIX] Fix truncated numbers.
* [CHANGE] Reorganize metrics names and types.

## 0.1.0 / 2015-05-05

### Initial release
