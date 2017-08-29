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

		name := strings.Replace(m[1], ".", "_", -1)
		value, err := strconv.Atoi(m[2])
		if err != nil {
			log.Warnf("failed to parse: %s", scanner.Text())
			continue
		}
		metrics = append(metrics, prometheus.MustNewConstMetric(
			newDesc(rocksDB, name, m[1]),
			prometheus.CounterValue,
			float64(value),
		))
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
