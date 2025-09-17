// Copyright 2025 The Prometheus Authors
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

// Scrape `information_schema.ROCKSDB_PERF_CONTEXT`.

package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const rocksdbPerfContextQuery = `
                SELECT
                  TABLE_SCHEMA, 
                  TABLE_NAME, 
                  ifnull(PARTITION_NAME, ''), 
                  STAT_TYPE,
                  VALUE
                  FROM information_schema.ROCKSDB_PERF_CONTEXT
                `

// Metric descriptors.
var informationSchemaRocksDBLabels = []string{"schema", "table", "part"}
var informationSchemaRocksDBPerfContextMetrics = map[string]struct {
	vtype prometheus.ValueType
	desc  *prometheus.Desc
}{
	"USER_KEY_COMPARISON_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_user_key_comparison_count"),
			"Total number of user key comparisons performed in binary search.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOCK_CACHE_HIT_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_block_cache_hit_count"),
			"Total number of block read operations from cache.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOCK_READ_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_block_read_count"),
			"Total number of block read operations from disk.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOCK_READ_BYTE": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_block_read_byte"),
			"Total number of bytes read from disk.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"GET_READ_BYTES": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_get_read_bytes"),
			"Number of bytes read during Get operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"MULTIGET_READ_BYTES": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_multiget_read_bytes"),
			"Number of bytes read during MultiGet operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"ITER_READ_BYTES": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_iter_read_bytes"),
			"Number of bytes read during iterator operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"INTERNAL_KEY_SKIPPED_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_internal_key_skipped_count"),
			"Count of internal keys skipped during operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"INTERNAL_DELETE_SKIPPED_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_internal_delete_skipped_count"),
			"Count of internal delete operations that were skipped.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"INTERNAL_RECENT_SKIPPED_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_internal_recent_skipped_count"),
			"Count of recently skipped internal operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"INTERNAL_MERGE_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_internal_merge_count"),
			"Total number of internal merge operations.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"GET_FROM_MEMTABLE_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_get_from_memtable_count"),
			"Number of Get operations served from the memtable.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"SEEK_ON_MEMTABLE_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_seek_on_memtable_count"),
			"Count of seek operations in the memtable.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"NEXT_ON_MEMTABLE_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_next_on_memtable_count"),
			"Count of next operations in the memtable.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"PREV_ON_MEMTABLE_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_prev_on_memtable_count"),
			"Count of previous operations in the memtable.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"SEEK_CHILD_SEEK_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_seek_child_seek_count"),
			"Count of child seek operations in RocksDB.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOOM_MEMTABLE_HIT_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_bloom_memtable_hit_count"),
			"Count of successful hits in the bloom filter for memtable searches.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOOM_MEMTABLE_MISS_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_bloom_memtable_miss_count"),
			"Count of misses in the bloom filter for memtable searches.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOOM_SST_HIT_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_bloom_sst_hit_count"),
			"Count of successful hits in the bloom filter for SSTable searches.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"BLOOM_SST_MISS_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_bloom_sst_miss_count"),
			"Count of misses in the bloom filter for SSTable searches.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"KEY_LOCK_WAIT_COUNT": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_key_lock_wait_count"),
			"Count of key lock wait events in RocksDB.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"IO_BYTES_WRITTEN": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_io_bytes_written"),
			"Total number of bytes written by I/O operations in RocksDB.",
			informationSchemaRocksDBLabels, nil,
		),
	},
	"IO_BYTES_READ": {
		prometheus.CounterValue,
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, informationSchema, "rocksdb_perf_context_io_bytes_read"),
			"Total number of bytes read by I/O operations in RocksDB.",
			informationSchemaRocksDBLabels, nil,
		),
	},
}

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
type ScrapeRocksDBPerfContext struct{}

// Name of the Scraper. Should be unique.
func (ScrapeRocksDBPerfContext) Name() string {
	return informationSchema + ".rocksdb_perf_context"
}

// Help describes the role of the Scraper.
func (ScrapeRocksDBPerfContext) Help() string {
	return "Collect metrics from information_schema.ROCKSDB_PERF_CONTEXT"
}

// Version of MySQL from which scraper is available.
func (ScrapeRocksDBPerfContext) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeRocksDBPerfContext) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()
	informationSchemaInnodbCmpMemRows, err := db.QueryContext(ctx, rocksdbPerfContextQuery)
	if err != nil {
		return err
	}
	defer informationSchemaInnodbCmpMemRows.Close()

	var (
		schema, table, part, stat string
		value                     float64
	)

	for informationSchemaInnodbCmpMemRows.Next() {
		if err := informationSchemaInnodbCmpMemRows.Scan(
			&schema, &table, &part, &stat, &value,
		); err != nil {
			return err
		}
		if v, ok := informationSchemaRocksDBPerfContextMetrics[stat]; ok {
			ch <- prometheus.MustNewConstMetric(v.desc, v.vtype, value, schema, table, part)
		}
	}
	return nil
}

// check interface
var _ Scraper = ScrapeRocksDBPerfContext{}
