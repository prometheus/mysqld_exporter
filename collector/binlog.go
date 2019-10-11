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

// Scrape `SHOW BINARY LOGS`

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-kit/kit/log"
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

// Version of MySQL from which scraper is available.
func (ScrapeBinlogSize) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeBinlogSize) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	var logBin uint8
	err := db.QueryRowContext(ctx, logbinQuery).Scan(&logBin)
	if err != nil {
		return err
	}
	// If log_bin is OFF, do not run SHOW BINARY LOGS which explicitly produces MySQL error
	if logBin == 0 {
		return nil
	}

	masterLogRows, err := db.QueryContext(ctx, binlogQuery)
	if err != nil {
		return err
	}
	defer masterLogRows.Close()

	var (
		size      uint64
		count     uint64
		filename  string
		filesize  uint64
		encrypted string
	)
	size = 0
	count = 0

	columns, err := masterLogRows.Columns()
	if err != nil {
		return err
	}
	columnCount := len(columns)

	for masterLogRows.Next() {
		switch columnCount {
		case 2:
			if err := masterLogRows.Scan(&filename, &filesize); err != nil {
				return nil
			}
		case 3:
			if err := masterLogRows.Scan(&filename, &filesize, &encrypted); err != nil {
				return nil
			}
		default:
			return fmt.Errorf("invalid number of columns: %q", columnCount)
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

// check interface
var _ Scraper = ScrapeBinlogSize{}
