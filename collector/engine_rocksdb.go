// Scrape `SHOW ENGINE ROCKSDB STATUS`.

package collector

import (
	"bufio"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	// Subsystem.
	rocksDB = "engine_rocksdb"
	// Query.
	engineRocksDBStatusQuery = `SHOW ENGINE ROCKSDB STATUS`
)

var (
	rocksDBCounter = regexp.MustCompile(`^([a-z\.]+) COUNT : (\d+)$`)

	// Map known counters to descriptions. Unknown counters will be skipped.
	engineRocksDBStatusTypes = map[string]*prometheus.Desc{
		"rocksdb.block.cache.miss":         newDesc(rocksDB, "block_cache_miss", "rocksdb.block.cache.miss"),
		"rocksdb.block.cache.hit":          newDesc(rocksDB, "block_cache_hit", "rocksdb.block.cache.hit"),
		"rocksdb.block.cache.add":          newDesc(rocksDB, "block_cache_add", "rocksdb.block.cache.add"),
		"rocksdb.block.cache.add.failures": newDesc(rocksDB, "block_cache_add_failures", "rocksdb.block.cache.add.failures"),

		"rocksdb.block.cache.data.miss": newDesc(rocksDB, "block_cache_data_miss", "rocksdb.block.cache.data.miss"),
		"rocksdb.block.cache.data.hit":  newDesc(rocksDB, "block_cache_data_hit", "rocksdb.block.cache.data.hit"),
		"rocksdb.block.cache.data.add":  newDesc(rocksDB, "block_cache_data_add", "rocksdb.block.cache.data.add"),

		"rocksdb.block.cache.index.miss": newDesc(rocksDB, "block_cache_index_miss", "rocksdb.block.cache.index.miss"),
		"rocksdb.block.cache.index.hit":  newDesc(rocksDB, "block_cache_index_hit", "rocksdb.block.cache.index.hit"),
		"rocksdb.block.cache.index.add":  newDesc(rocksDB, "block_cache_index_add", "rocksdb.block.cache.index.add"),

		"rocksdb.block.cache.index.bytes.insert": newDesc(rocksDB, "block_cache_index_bytes_insert", "rocksdb.block.cache.index.bytes.insert"),
		"rocksdb.block.cache.index.bytes.evict":  newDesc(rocksDB, "block_cache_index_bytes_evict", "rocksdb.block.cache.index.bytes.evict"),

		"rocksdb.block.cache.filter.miss": newDesc(rocksDB, "block_cache_filter_miss", "rocksdb.block.cache.filter.miss"),
		"rocksdb.block.cache.filter.hit":  newDesc(rocksDB, "block_cache_filter_hit", "rocksdb.block.cache.filter.hit"),
		"rocksdb.block.cache.filter.add":  newDesc(rocksDB, "block_cache_filter_add", "rocksdb.block.cache.filter.add"),

		"rocksdb.block.cache.filter.bytes.insert": newDesc(rocksDB, "block_cache_filter_bytes_insert", "rocksdb.block.cache.filter.bytes.insert"),
		"rocksdb.block.cache.filter.bytes.evict":  newDesc(rocksDB, "block_cache_filter_bytes_evict", "rocksdb.block.cache.filter.bytes.evict"),

		"rocksdb.block.cache.bytes.read":  newDesc(rocksDB, "block_cache_bytes_read", "rocksdb.block.cache.bytes.read"),
		"rocksdb.block.cache.bytes.write": newDesc(rocksDB, "block_cache_bytes_write", "rocksdb.block.cache.bytes.write"),

		"rocksdb.block.cache.data.bytes.insert": newDesc(rocksDB, "block_cache_data_bytes_insert", "rocksdb.block.cache.data.bytes.insert"),

		"rocksdb.bloom.filter.useful": newDesc(rocksDB, "bloom_filter_useful", "rocksdb.bloom.filter.useful"),

		"rocksdb.memtable.miss": newDesc(rocksDB, "memtable_miss", "rocksdb.memtable.miss"),
		"rocksdb.memtable.hit":  newDesc(rocksDB, "memtable_hit", "rocksdb.memtable.hit"),

		"rocksdb.l0.hit": newDesc(rocksDB, "l0_hit", "rocksdb.l0.hit"),
		"rocksdb.l1.hit": newDesc(rocksDB, "l1_hit", "rocksdb.l1.hit"),

		"rocksdb.number.keys.read":    newDesc(rocksDB, "number_keys_read", "rocksdb.number.keys.read"),
		"rocksdb.number.keys.written": newDesc(rocksDB, "number_keys_written", "rocksdb.number.keys.written"),
		"rocksdb.number.keys.updated": newDesc(rocksDB, "number_keys_updated", "rocksdb.number.keys.updated"),

		"rocksdb.bytes.read":    newDesc(rocksDB, "bytes_read", "rocksdb.bytes.read"),
		"rocksdb.bytes.written": newDesc(rocksDB, "bytes_written", "rocksdb.bytes.written"),

		"rocksdb.number.db.seek":       newDesc(rocksDB, "number_db_seek", "rocksdb.number.db.seek"),
		"rocksdb.number.db.seek.found": newDesc(rocksDB, "number_db_seek_found", "rocksdb.number.db.seek.found"),
		"rocksdb.number.db.next":       newDesc(rocksDB, "number_db_next", "rocksdb.number.db.next"),
		"rocksdb.number.db.next.found": newDesc(rocksDB, "number_db_next_found", "rocksdb.number.db.next.found"),
		"rocksdb.number.db.prev":       newDesc(rocksDB, "number_db_prev", "rocksdb.number.db.prev"),
		"rocksdb.number.db.prev.found": newDesc(rocksDB, "number_db_prev_found", "rocksdb.number.db.prev.found"),

		"rocksdb.db.iter.bytes.read":       newDesc(rocksDB, "db_iter_bytes_read", "rocksdb.db.iter.bytes.read"),
		"rocksdb.number.reseeks.iteration": newDesc(rocksDB, "number_reseeks_iteration", "rocksdb.number.reseeks.iteration"),

		"rocksdb.wal.synced": newDesc(rocksDB, "wal_synced", "rocksdb.wal.synced"),
		"rocksdb.wal.bytes":  newDesc(rocksDB, "wal_bytes", "rocksdb.wal.bytes"),

		"rocksdb.no.file.opens":  newDesc(rocksDB, "no_file_opens", "rocksdb.no.file.opens"),
		"rocksdb.no.file.closes": newDesc(rocksDB, "no_file_closes", "rocksdb.no.file.closes"),
		"rocksdb.no.file.errors": newDesc(rocksDB, "no_file_errors", "rocksdb.no.file.errors"),

		"rocksdb.write.self":    newDesc(rocksDB, "write_self", "rocksdb.write.self"),
		"rocksdb.write.other":   newDesc(rocksDB, "write_other", "rocksdb.write.other"),
		"rocksdb.write.timeout": newDesc(rocksDB, "write_timeout", "rocksdb.write.timeout"),
		"rocksdb.write.wal":     newDesc(rocksDB, "write_wal", "rocksdb.write.wal"),
	}
)

func parseRocksDBStatistics(data string) ([]prometheus.Metric, error) {
	var metrics []prometheus.Metric
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		m := rocksDBCounter.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}

		value, err := strconv.Atoi(m[2])
		if err != nil {
			log.Warnf("failed to parse: %s", scanner.Text())
			continue
		}
		description := engineRocksDBStatusTypes[m[1]]
		if description == nil {
			continue
		}
		metrics = append(metrics, prometheus.MustNewConstMetric(description, prometheus.CounterValue, float64(value)))
	}
	return metrics, scanner.Err()
}

// ScrapeEngineRocksDBStatus scrapes from `SHOW ENGINE ROCKSDB STATUS`.
func ScrapeEngineRocksDBStatus(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(engineRocksDBStatusQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var typeCol, nameCol, statusCol string
	for rows.Next() {
		if err := rows.Scan(&typeCol, &nameCol, &statusCol); err != nil {
			return err
		}

		if typeCol == "STATISTICS" && nameCol == "rocksdb" {
			metrics, err := parseRocksDBStatistics(statusCol)
			for _, m := range metrics {
				ch <- m
			}
			if err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
