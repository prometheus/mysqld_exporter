## v0.10.0 / 2017-04-25

BREAKING CHANGES:
* `mysql_slave_...` metrics now include an additional `connection_name` label to support mariadb multi-source replication. (#178)

* [FEATURE] Add read/write query response time #166
* [FEATURE] Add Galera gcache size metric #169
* [FEATURE] Add MariaDB multi source replication support #178
* [FEATURE] Implement heartbeat metrics #183
* [FEATURE] Add basic file_summary_by_instance metrics #189
* [BUGFIX] Workaround MySQL bug 79533 #173

## 0.9.0 / 2016-09-26

BREAKING CHANGES:
* InnoDB buffer pool page stats have been renamed/fixed to better support aggregations (#130)

* [FEATURE] scrape slave status for multisource replication #134
* [FEATURE] Add client statistics support (+ add tests on users & clients statistics) #138
* [IMPROVEMENT] Consistency of error logging. #144
* [IMPROVEMENT] Add label aggregation for innodb buffer metrics #130
* [IMPROVEMENT] Improved and fixed user/client statistics #149
* [FEATURE] Added the last binlog file number metric. #152
* [MISC] Add an example recording rules file #156
* [FEATURE] Added PXC/Galera info metrics. #155
* [FEATURE] Added metrics from SHOW ENGINE INNODB STATUS. #160
* [IMPROVEMENT] Fix wsrep_cluster_status #146


## 0.8.1 / 2016-05-05

* [BUGFIX] Fix collect.info_schema.innodb_tablespaces #119
* [BUGFIX] Fix SLAVE STATUS "Connecting" #125
* [MISC] New release process using docker, circleci and a centralized building tool #120
* [MISC] Typos #121

## 0.8.0 / 2016-04-19

BREAKING CHANGES:
* global status `innodb_buffer_pool_pages` have been renamed/labeled.
* innodb metrics `buffer_page_io` have been renamed/labeled.

* [MISC] Add Travis CI automatic testing.
* [MISC] Refactor mysqld_exporter.go into collector package.
* [FEATURE] Add `mysql_up` metric (PR #99)
* [FEATURE] Collect time metrics for processlist (PR #87)
* [CHANGE] Separate innodb_buffer_pool_pages status metrics (PR #101)
* [FEATURE] Added metrics from SHOW ENGINE TOKUDB STATUS (PR #103)
* [CHANGE] Add special handling of "buffer_page_io" subsystem (PR #115)
* [FEATURE] Add collector for innodb_sys_tablespaces (PR #116)

## 0.7.1 / 2016-02-16

* [IMPROVEMENT] Soft error on collector failure (PR #84)
* [BUGFIX] Fix innodb_metrics collector (PR #85)
* [BUGFIX] Parse auto increment values and maximum as float64 (PR #88)

## 0.7.0 / 2016-02-12

BREAKING CHANGES:
* Global status metrics for "handlers" have been renamed

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
* [BUGFIX] Check log_bin before running SHOW BINARY LOGS (PR #74)
* [BUGFIX] Fixed uint for scrapeInnodbMetrics() and gofmt (PR #81)

## 0.6.0 / 2015-10-28

BREAKING CHANGES:
* The digest_text mapping metric has been removed, now included in all digest metrics (PR #50)
* Flags for timing metrics have been removed, now included with related counter flag (PR #48)

* [FEATURE] New collector for metrics from information_schema.processlist (PR #34)
* [FEATURE] New collector for binlog counts/sizes (PR #35)
* [FEATURE] New collector for performance_schema.{file_summary_by_event_name,events_waits_summary_global_by_event_name} (PR #49)
* [FEATURE] New collector for information_schema.tables (PR #51)
* [IMPROVEMENT] All collection methods now have enable flags (PR #46)
* [IMPROVEMENT] Consolidate performance_schema metrics flags (PR #48)
* [IMPROVEMENT] Removed need for digest_text mapping metric (PR #50)
* [IMPROVEMENT] Update docs (PR #52)

## 0.5.0 / 2015-09-22

* [FEATURE] Add metrics for table locks
* [BUGFIX] Use uint64 to prevent int64 overflow
* [BUGFIX] Correct picsecond times to correct second values

## 0.4.0 / 2015-09-21

* [CHANGE] Limit events_statements to recently used
* [FEATURE] Add digest_text mapping metric
* [IMPROVEMENT] General refactoring

## 0.3.0 / 2015-08-31

BREAKING CHANGES: Most metrics have been prefixed with Prometheus subsystem names
                  to avoid conflicts between different collection methods.

* [BUGFIX] Separate slave_status and global_status into separate subsystems.
* [IMPROVEMENT] Refactor metrics creation.
* [IMPROVEMENT] Add support for performance_schema.table_io_waits_summary_by_table collection.
* [IMPROVEMENT] Add support for performance_schema.table_io_waits_summary_by_index_usage collection.
* [IMPROVEMENT] Add support for performance_schema.events_statements_summary_by_digest collection.
* [IMPROVEMENT] Add support for Percona userstats output collection.
* [IMPROVEMENT] Add support for auto_increment column metrics collection.
* [IMPROVEMENT] Add support for `SHOW GLOBAL VARIABLES` metrics collection.

## 0.2.0 / 2015-06-24

BREAKING CHANGES: Logging-related flags have changed. Metric names have changed.

* [IMPROVEMENT] Add Docker support.
* [CHANGE] Switch logging to Prometheus' logging library.
* [BUGFIX] Fix slave status parsing.
* [BUGFIX] Fix truncated numbers.
* [CHANGE] Reorganize metrics names and types.

## 0.1.0 / 2015-05-05

* Initial release
