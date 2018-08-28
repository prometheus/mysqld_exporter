// Scrape `information_schema.client_statistics`.

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const innodbCmpQuery = `
                SELECT
                  page_size, compress_ops, compress_ops_ok, compress_time, uncompress_ops, uncompress_time
                  FROM information_schema.INNODB_CMP
                `

var (
	// Map known innodb_cmp values to types. Unknown types will be mapped as
	// untyped.
	informationSchemaInnodbCmpTypes = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"compress_ops": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_ops_total"),
				"Number of times a B-tree page of the size PAGE_SIZE has been compressed.",
				[]string{"page_size"}, nil)},
		"compress_ops_ok": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_ops_ok_total"),
				"Number of times a B-tree page of the size PAGE_SIZE has been successfully compressed.",
				[]string{"page_size"}, nil)},
		"compress_time": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_time_seconds_total"),
				"Total time in seconds spent in attempts to compress B-tree pages.",
				[]string{"page_size"}, nil)},
		"uncompress_ops": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_uncompress_ops_total"),
				"Number of times a B-tree page has been uncompressed.",
				[]string{"page_size"}, nil)},
		"uncompress_time": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_uncompress_time_seconds_total"),
				"Total time in seconds spent in uncompressing B-tree pages.",
				[]string{"page_size"}, nil)},
	}
)

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
type ScrapeInnodbCmp struct{}

// Name of the Scraper.
func (ScrapeInnodbCmp) Name() string {
	return "info_schema.innodb_cmp"
}

// Help returns additional information about Scraper.
func (ScrapeInnodbCmp) Help() string {
	return "Please set next variables SET GLOBAL innodb_file_per_table=1;SET GLOBAL innodb_file_format=Barracuda;"
}

// Version of MySQL from which scraper is available.
func (ScrapeInnodbCmp) Version() float64 {
	return 5.5
}

// Scrape collects data.
func (ScrapeInnodbCmp) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	informationSchemaInnodbCmpRows, err := db.QueryContext(ctx, innodbCmpQuery)
	if err != nil {
		log.Debugln("INNODB_CMP stats are not available.")
		return err
	}
	defer informationSchemaInnodbCmpRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaInnodbCmpRows.Columns()
	if err != nil {
		log.Debugln("INNODB_CMP stats are not available.")
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
		// cient, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metricType, ok := informationSchemaInnodbCmpTypes[columnName]; ok {
				ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(clientStatData[idx]), client)
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, fmt.Sprintf("innodb_cmp_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"page_size"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(clientStatData[idx]), client)
			}
		}
	}
	return nil
}
