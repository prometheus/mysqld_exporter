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

// Scrape `information_schema.INNODB_CMP`.

package perconacollector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	cl "github.com/percona/mysqld_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
)

const innodbCmpQuery = `
		SELECT
		  page_size, compress_ops, compress_ops_ok, compress_time, uncompress_ops, uncompress_time
		  FROM information_schema.innodb_cmp
		`

// Map known innodb_cmp values to types. Unknown types will be mapped as
// untyped.
var informationSchemaInnodbCmpTypes = map[string]struct {
	vtype prometheus.ValueType
	desc  *prometheus.Desc
}{
	"compress_ops": {
		prometheus.CounterValue,
		prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, "innodb_cmp_compress_ops_total"),
			"Number of times a B-tree page of the size PAGE_SIZE has been compressed.",
			[]string{"page_size"}, nil),
	},
	"compress_ops_ok": {
		prometheus.CounterValue,
		prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, "innodb_cmp_compress_ops_ok_total"),
			"Number of times a B-tree page of the size PAGE_SIZE has been successfully compressed.",
			[]string{"page_size"}, nil),
	},
	"compress_time": {
		prometheus.CounterValue,
		prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, "innodb_cmp_compress_time_seconds_total"),
			"Total time in seconds spent in attempts to compress B-tree pages.",
			[]string{"page_size"}, nil),
	},
	"uncompress_ops": {
		prometheus.CounterValue,
		prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, "innodb_cmp_uncompress_ops_total"),
			"Number of times a B-tree page has been uncompressed.",
			[]string{"page_size"}, nil),
	},
	"uncompress_time": {
		prometheus.CounterValue,
		prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, "innodb_cmp_uncompress_time_seconds_total"),
			"Total time in seconds spent in uncompressing B-tree pages.",
			[]string{"page_size"}, nil),
	},
}

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
type ScrapeInnodbCmp struct{}

// Name of the Scraper. Should be unique.
func (ScrapeInnodbCmp) Name() string {
	return "info_schema.innodb_cmp"
}

// Help describes the role of the Scraper.
func (ScrapeInnodbCmp) Help() string {
	return "Please set next variables SET GLOBAL innodb_file_per_table=1;SET GLOBAL innodb_file_format=Barracuda;"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbCmp) Version() float64 {
	return 5.5
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeInnodbCmp) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	informationSchemaInnodbCmpRows, err := db.QueryContext(ctx, innodbCmpQuery)
	if err != nil {
		level.Debug(logger).Log("msg", "INNODB_CMP stats are not available.", "error", err)
		return err
	}
	defer informationSchemaInnodbCmpRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaInnodbCmpRows.Columns()
	if err != nil {
		level.Debug(logger).Log("msg", "INNODB_CMP stats are not available.", "error", err)
		return err
	}

	var (
		client             string                                // Holds the client name, which should be in column 0.
		clientStatData     = make([]float64, len(columnNames)-1) // 1 less because of the client column.
		clientStatScanArgs = make([]interface{}, len(columnNames))
	)

	clientStatScanArgs[0] = &client
	for i := range clientStatData {
		clientStatScanArgs[i+1] = &clientStatData[i]
	}

	for informationSchemaInnodbCmpRows.Next() {
		if err := informationSchemaInnodbCmpRows.Scan(clientStatScanArgs...); err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// client, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metricType, ok := informationSchemaInnodbCmpTypes[columnName]; ok {
				ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientStatData[idx]), client)
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(cl.Namespace, cl.InformationSchema, fmt.Sprintf("innodb_cmp_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"page_size"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(clientStatData[idx]), client)
			}
		}
	}

	return nil
}

// check interface
var _ cl.Scraper = ScrapeInnodbCmp{}
