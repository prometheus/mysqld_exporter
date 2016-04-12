// Scrape `SHOW BINARY LOGS`

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem.
	binlog = "binlog"
	// Queries.
	logbinQuery = `SELECT @@log_bin`
	binlogQuery = `SHOW BINARY LOGS`
)

// Metric descriptors.
var (
	binlogSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, binlog, "size_bytes"),
		"Combined size of all registered binlog files.",
		[]string{}, nil,
	)
	binlogFilesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, binlog, "files"),
		"Number of registered binlog files.",
		[]string{}, nil,
	)
)

func ScrapeBinlogSize(db *sql.DB, ch chan<- prometheus.Metric) error {
	var logBin uint8
	err := db.QueryRow(logbinQuery).Scan(&logBin)
	if err != nil {
		return err
	}
	// If log_bin is OFF, do not run SHOW BINARY LOGS which explicitly produces MySQL error
	if logBin == 0 {
		return nil
	}

	masterLogRows, err := db.Query(binlogQuery)
	if err != nil {
		return err
	}
	defer masterLogRows.Close()

	var (
		size     uint64
		count    uint64
		filename string
		filesize uint64
	)
	size = 0
	count = 0

	for masterLogRows.Next() {
		if err := masterLogRows.Scan(&filename, &filesize); err != nil {
			return nil
		}
		size += filesize
		count++
	}

	ch <- prometheus.MustNewConstMetric(
		binlogSizeDesc, prometheus.GaugeValue, float64(size),
	)
	ch <- prometheus.MustNewConstMetric(
		binlogFilesDesc, prometheus.GaugeValue, float64(count),
	)

	return nil
}
