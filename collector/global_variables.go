// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Scrape `SHOW GLOBAL VARIABLES`.

package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Metric subsystem
	globalVariables = "global_variables"
	// Metric SQL Queries.
	globalVariablesQuery = `SHOW GLOBAL VARIABLES`
)

var (
	// Map known global variables to help strings. Unknown will be mapped to generic gauges.
	globalVariablesHelp = map[string]string{
		// https://github.com/facebook/mysql-5.6/wiki/New-MySQL-RocksDB-Server-Variables
		"rocksdb_access_hint_on_compaction_start":         "File access pattern once a compaction is started, applied to all input files of a compaction.",
		"rocksdb_advise_random_on_open":                   "Hint of random access to the filesystem when a data file is opened.",
		"rocksdb_allow_concurrent_memtable_write":         "Allow multi-writers to update memtables in parallel.",
		"rocksdb_allow_mmap_reads":                        "Allow the OS to mmap a data file for reads.",
		"rocksdb_allow_mmap_writes":                       "Allow the OS to mmap a data file for writes.",
		"rocksdb_block_cache_size":                        "Size of the LRU block cache in RocksDB. This memory is reserved for the block cache, which is in addition to any filesystem caching that may occur.",
		"rocksdb_block_restart_interval":                  "Number of keys for each set of delta encoded data.",
		"rocksdb_block_size_deviation":                    "If the percentage of free space in the current data block (size specified in rocksdb-block-size) is less than this amount, close the block (and write record to new block).",
		"rocksdb_block_size":                              "Size of the data block for reading sst files.",
		"rocksdb_bulk_load_size":                          "Sets the number of keys to accumulate before committing them to the storage engine during bulk loading.",
		"rocksdb_bulk_load":                               "When set, MyRocks will ignore checking keys for uniqueness or acquiring locks during transactions. This option should only be used when the application is certain there are no row conflicts, such as when setting up a new MyRocks instance from an existing MySQL dump.",
		"rocksdb_bytes_per_sync":                          "Enables the OS to sync out file writes as data files are created.",
		"rocksdb_cache_index_and_filter_blocks":           "Requests RocksDB to use the block cache for caching the index and bloomfilter data blocks from each data file. If this is not set, RocksDB will allocate additional memory to maintain these data blocks.",
		"rocksdb_checksums_pct":                           "Sets the percentage of rows to calculate and set MyRocks checksums.",
		"rocksdb_collect_sst_properties":                  "Enables collecting statistics of each data file for improving optimizer behavior.",
		"rocksdb_commit_in_the_middle":                    "Commit rows implicitly every rocksdb-bulk-load-size, during bulk load/insert/update/deletes.",
		"rocksdb_compaction_readahead_size":               "When non-zero, bigger reads are performed during compaction. Useful if running RocksDB on spinning disks, compaction will do sequential instead of random reads.",
		"rocksdb_compaction_sequential_deletes_count_sd":  "If enabled, factor in single deletes as part of rocksdb-compaction-sequential-deletes.",
		"rocksdb_compaction_sequential_deletes_file_size": "Threshold to trigger compaction if the number of sequential keys that are all delete markers exceed this value. While this compaction helps reduce request latency by removing delete markers, it can increase write rates of RocksDB.",
		"rocksdb_compaction_sequential_deletes_window":    "Threshold to trigger compaction if, within a sliding window of keys, there exists this parameter's number of delete marker.",
		"rocksdb_compaction_sequential_deletes":           "Enables triggering of compaction when the number of delete markers in a data file exceeds a certain threshold. Depending on workload patterns, RocksDB can potentially maintain large numbers of delete markers and increase latency of all queries.",
		"rocksdb_create_if_missing":                       "Allows creating the RocksDB database if it does not exist.",
		"rocksdb_create_missing_column_families":          "Allows creating new column families if they did not exist.",
		"rocksdb_db_write_buffer_size":                    "Size of the memtable used to store writes within RocksDB. This is the size per column family. Once this size is reached, a flush of the memtable to persistent media occurs.",
		"rocksdb_deadlock_detect":                         "Enables deadlock detection in RocksDB.",
		"rocksdb_debug_optimizer_no_zero_cardinality":     "Test only to prevent MyRocks from calculating cardinality.",
		"rocksdb_delayed_write_rate":                      "When RocksDB hits the soft limits/thresholds for writes, such as soft_pending_compaction_bytes_limit being hit, or level0_slowdown_writes_trigger being hit, RocksDB will slow the write rate down to the value of this parameter as bytes/second.",
		"rocksdb_delete_obsolete_files_period_micros":     "The periodicity of when obsolete files get deleted, but does not affect files removed through compaction.",
		"rocksdb_enable_bulk_load_api":                    "Enables using the SSTFileWriter feature in RocksDB, which bypasses the memtable, but this requires keys to be inserted into the table in either ascending or descending order. If disabled, bulk loading uses the normal write path via the memtable and does not keys to be inserted in any order.",
		"rocksdb_enable_thread_tracking":                  "Set to allow RocksDB to track the status of threads accessing the database.",
		"rocksdb_enable_write_thread_adaptive_yield":      "Set to allow RocksDB write batch group leader to wait up to the max time allowed before blocking on a mutex, allowing an increase in throughput for concurrent workloads.",
		"rocksdb_error_if_exists":                         "If set, reports an error if an existing database already exists.",
		"rocksdb_flush_log_at_trx_commit":                 "Sync'ing on transaction commit similar to innodb-flush-log-at-trx-commit: 0 - never sync, 1 - always sync, 2 - sync based on a timer controlled via rocksdb-background-sync",
		"rocksdb_flush_memtable_on_analyze":               "When analyze table is run, determines of the memtable should be flushed so that data in the memtable is also used for calculating stats.",
		"rocksdb_force_compute_memtable_stats":            "When enabled, also include data in the memtables for index statistics calculations used by the query optimizer. Greater accuracy, but requires more cpu.",
		"rocksdb_force_flush_memtable_now":                "Triggers MyRocks to flush the memtables out to the data files.",
		"rocksdb_force_index_records_in_range":            "When force index is used, a non-zero value here will be used as the number of rows to be returned to the query optimizer when trying to determine the estimated number of rows.",
		"rocksdb_hash_index_allow_collision":              "Enables RocksDB to allow hashes to collide (uses less memory). Otherwise, the full prefix is stored to prevent hash collisions.",
		"rocksdb_keep_log_file_num":                       "Sets the maximum number of info LOG files to keep around.",
		"rocksdb_lock_scanned_rows":                       "If enabled, rows that are scanned during UPDATE remain locked even if they have not been updated.",
		"rocksdb_lock_wait_timeout":                       "Sets the number of seconds MyRocks will wait to acquire a row lock before aborting the request.",
		"rocksdb_log_file_time_to_roll":                   "Sets the number of seconds a info LOG file captures before rolling to a new LOG file.",
		"rocksdb_manifest_preallocation_size":             "Sets the number of bytes to preallocate for the MANIFEST file in RocksDB and reduce possible random I/O on XFS. MANIFEST files are used to store information about column families, levels, active files, etc.",
		"rocksdb_max_open_files":                          "Sets a limit on the maximum number of file handles opened by RocksDB.",
		"rocksdb_max_row_locks":                           "Sets a limit on the maximum number of row locks held by a transaction before failing it.",
		"rocksdb_max_subcompactions":                      "For each compaction job, the maximum threads that will work on it simultaneously (i.e. subcompactions). A value of 1 means no subcompactions.",
		"rocksdb_max_total_wal_size":                      "Sets a limit on the maximum size of WAL files kept around. Once this limit is hit, RocksDB will force the flushing of memtables to reduce the size of WAL files.",
		"rocksdb_merge_buf_size":                          "Size (in bytes) of the merge buffers used to accumulate data during secondary key creation. During secondary key creation the data, we avoid updating the new indexes through the memtable and L0 by writing new entries directly to the lowest level in the database. This requires the values to be sorted so we use a merge/sort algorithm. This setting controls how large the merge buffers are. The default is 64Mb.",
		"rocksdb_merge_combine_read_size":                 "Size (in bytes) of the merge combine buffer used in the merge/sort algorithm as described in rocksdb-merge-buf-size.",
		"rocksdb_new_table_reader_for_compaction_inputs":  "Indicates whether RocksDB should create a new file descriptor and table reader for each compaction input. Doing so may use more memory but may allow pre-fetch options to be specified for compaction input files without impacting table readers used for user queries.",
		"rocksdb_no_block_cache":                          "Disables using the block cache for a column family.",
		"rocksdb_paranoid_checks":                         "Forces RocksDB to re-read a data file that was just created to verify correctness.",
		"rocksdb_pause_background_work":                   "Test only to start and stop all background compactions within RocksDB.",
		"rocksdb_perf_context_level":                      "Sets the level of information to capture via the perf context plugins.",
		"rocksdb_persistent_cache_size_mb":                "The size (in Mb) to allocate to the RocksDB persistent cache if desired.",
		"rocksdb_pin_l0_filter_and_index_blocks_in_cache": "If rocksdb-cache-index-and-filter-blocks is true then this controls whether RocksDB 'pins' the filter and index blocks in the cache.",
		"rocksdb_print_snapshot_conflict_queries":         "If this is true, MyRocks will log queries that generate snapshot conflicts into the .err log.",
		"rocksdb_rate_limiter_bytes_per_sec":              "Controls the rate at which RocksDB is allowed to write to media via memtable flushes and compaction.",
		"rocksdb_records_in_range":                        "Test only to override the value returned by records-in-range.",
		"rocksdb_seconds_between_stat_computes":           "Sets the number of seconds between recomputation of table statistics for the optimizer.",
		"rocksdb_signal_drop_index_thread":                "Test only to signal the MyRocks drop index thread.",
		"rocksdb_skip_bloom_filter_on_read":               "Indicates whether the bloom filters should be skipped on reads.",
		"rocksdb_skip_fill_cache":                         "Requests MyRocks to skip caching data on read requests.",
		"rocksdb_stats_dump_period_sec":                   "Sets the number of seconds to perform a RocksDB stats dump to the info LOG files.",
		"rocksdb_store_row_debug_checksums":               "Include checksums when writing index/table records.",
		"rocksdb_strict_collation_check":                  "Enables MyRocks to check and verify table indexes have the proper collation settings.",
		"rocksdb_table_cache_numshardbits":                "Sets the number of table caches within RocksDB.",
		"rocksdb_use_adaptive_mutex":                      "Enables adaptive mutexes in RocksDB which spins in user space before resorting to the kernel.",
		"rocksdb_use_direct_reads":                        "Enable direct IO when opening a file for read/write. This means that data will not be cached or buffered.",
		"rocksdb_use_fsync":                               "Requires RocksDB to use fsync instead of fdatasync when requesting a sync of a data file.",
		"rocksdb_validate_tables":                         "Requires MyRocks to verify all of MySQL's .frm files match tables stored in RocksDB.",
		"rocksdb_verify_row_debug_checksums":              "Verify checksums when reading index/table records.",
		"rocksdb_wal_bytes_per_sync":                      "Controls the rate at which RocksDB writes out WAL file data.",
		"rocksdb_wal_recovery_mode":                       "Sets RocksDB's level of tolerance when recovering the WAL files after a system crash.",
		"rocksdb_wal_size_limit_mb":                       "Maximum size the RocksDB WAL is allow to grow to. When this size is exceeded rocksdb attempts to flush sufficient memtables to allow for the deletion of the oldest log.",
		"rocksdb_wal_ttl_seconds":                         "No WAL file older than this value should exist.",
		"rocksdb_whole_key_filtering":                     "Enables the bloomfilter to use the whole key for filtering instead of just the prefix. In order for this to be efficient, lookups should use the whole key for matching.",
		"rocksdb_write_disable_wal":                       "Disables logging data to the WAL files. Useful for bulk loading.",
		"rocksdb_write_ignore_missing_column_families":    "If 1, then writes to column families that do not exist is ignored by RocksDB.",
	}
)

// ScrapeGlobalVariables collects from `SHOW GLOBAL VARIABLES`.
type ScrapeGlobalVariables struct{}

// Name of the Scraper. Should be unique.
func (ScrapeGlobalVariables) Name() string {
	return globalVariables
}

// Help describes the role of the Scraper.
func (ScrapeGlobalVariables) Help() string {
	return "Collect from SHOW GLOBAL VARIABLES"
}

// Version of MySQL from which scraper is available.
func (ScrapeGlobalVariables) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeGlobalVariables) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	globalVariablesRows, err := db.QueryContext(ctx, globalVariablesQuery)
	if err != nil {
		return err
	}
	defer globalVariablesRows.Close()

	var key string
	var val sql.RawBytes
	var textItems = map[string]string{
		"innodb_version":         "",
		"version":                "",
		"version_comment":        "",
		"wsrep_cluster_name":     "",
		"wsrep_provider_options": "",
	}

	for globalVariablesRows.Next() {
		if err = globalVariablesRows.Scan(&key, &val); err != nil {
			return err
		}

		key = validPrometheusName(key)
		if floatVal, ok := parseStatus(val); ok {
			help := globalVariablesHelp[key]
			if help == "" {
				help = "Generic gauge metric from SHOW GLOBAL VARIABLES."
			}
			ch <- prometheus.MustNewConstMetric(
				newDesc(globalVariables, key, help),
				prometheus.GaugeValue,
				floatVal,
			)
			continue
		}

		if _, ok := textItems[key]; ok {
			textItems[key] = string(val)
		}
	}

	// mysql_version_info metric.
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "version", "info"), "MySQL version and distribution.",
			[]string{"innodb_version", "version", "version_comment"}, nil),
		prometheus.GaugeValue, 1, textItems["innodb_version"], textItems["version"], textItems["version_comment"],
	)

	// mysql_galera_variables_info metric.
	if textItems["wsrep_cluster_name"] != "" {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "galera", "variables_info"), "PXC/Galera variables information.",
				[]string{"wsrep_cluster_name"}, nil),
			prometheus.GaugeValue, 1, textItems["wsrep_cluster_name"],
		)
	}

	// mysql_galera_gcache_size_bytes metric.
	if textItems["wsrep_provider_options"] != "" {
		ch <- prometheus.MustNewConstMetric(
			newDesc("galera", "gcache_size_bytes", "PXC/Galera gcache size."),
			prometheus.GaugeValue,
			parseWsrepProviderOptions(textItems["wsrep_provider_options"]),
		)
	}

	return nil
}

// parseWsrepProviderOptions parse wsrep_provider_options to get gcache.size in bytes.
func parseWsrepProviderOptions(opts string) float64 {
	var val float64
	r, _ := regexp.Compile(`gcache.size = (\d+)([MG]?);`)
	data := r.FindStringSubmatch(opts)
	if data == nil {
		return 0
	}

	val, _ = strconv.ParseFloat(data[1], 64)
	switch data[2] {
	case "M":
		val = val * 1024 * 1024
	case "G":
		val = val * 1024 * 1024 * 1024
	}

	return val
}

func validPrometheusName(s string) string {
	nameRe := regexp.MustCompile("([^a-zA-Z0-9_])")
	s = nameRe.ReplaceAllString(s, "_")
	s = strings.ToLower(s)
	return s
}

// check interface
var _ Scraper = ScrapeGlobalVariables{}
