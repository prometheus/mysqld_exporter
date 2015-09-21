## 0.4.0 / 2015-09-21

[CHANGE] Limit events_statements to recently used
[FEATURE] Add digest_text mapping metric
[IMPROVEMENT] General refactoring

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
