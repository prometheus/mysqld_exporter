// Scrape `SHOW BINARY LOGS`

package collector

import (
	"database/sql"
	"strconv"
	"strings"

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
	binlogFileNumberDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, binlog, "file_number"),
		"The last binlog file number.",
		[]string{}, nil,
	)
)

// ScrapeBinlogSize colects from `SHOW BINARY LOGS`.
type ScrapeBinlogSize struct{}

// Name of the Scraper. Should be unique.
func (ScrapeBinlogSize) Name() string {
	return "binlog_size"
}

// Help describes the role of the Scraper.
func (ScrapeBinlogSize) Help() string {
	return "Collect the current size of all registered binlog files"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeBinlogSize) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
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
	// The last row contains the last binlog file number.
	value, _ := strconv.ParseFloat(strings.Split(filename, ".")[1], 64)
	ch <- prometheus.MustNewConstMetric(
		binlogFileNumberDesc, prometheus.GaugeValue, value,
	)

	return nil
}
