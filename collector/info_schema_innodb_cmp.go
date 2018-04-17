// Scrape `information_schema.INNODB_CMP`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

const innodbCmpQuery = `
		SELECT 
		  page_size, compress_ops, compress_ops_ok, compress_time, uncompress_ops, uncompress_time 
		  FROM information_schema.innodb_cmp
		`

var (
	infoSchemaInnodbCmpCompressOps = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_ops_total"),
		"Number of times a B-tree page of the size PAGE_SIZE has been compressed.",
		[]string{"page_size"}, nil,
	)
	infoSchemaInnodbCmpCompressOpsOk = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_ops_ok_total"),
		"Number of times a B-tree page of the size PAGE_SIZE has been successfully compressed.",
		[]string{"page_size"}, nil,
	)
	infoSchemaInnodbCmpCompressTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_compress_time_seconds_total"),
		"Total time in seconds spent in attempts to compress B-tree pages.",
		[]string{"page_size"}, nil,
	)
	infoSchemaInnodbCmpUncompressOps = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_uncompress_ops_total"),
		"Number of times a B-tree page of the size PAGE_SIZE has been uncompressed.",
		[]string{"page_size"}, nil,
	)
	infoSchemaInnodbCmpUncompressTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "innodb_cmp_uncompress_time_seconds_total"),
		"Total time in seconds spent in uncompressing B-tree pages.",
		[]string{"page_size"}, nil,
	)
)

// ScrapeInnodbCmp collects from `information_schema.innodb_cmp`.
func ScrapeInnodbCmp(db *sql.DB, ch chan<- prometheus.Metric) error {

	informationSchemaInnodbCmpRows, err := db.Query(innodbCmpQuery)
	if err != nil {
		return err
	}
	defer informationSchemaInnodbCmpRows.Close()

	// The client column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the client column, clientStatData[0] maps to columnNames[1] when reading off the metrics
	// (because clientStatScanArgs is mapped as [ &client, &clientData[0], &clientData[1] ... &clientdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	columnNames, err := informationSchemaInnodbCmpRows.Columns()
	if err != nil {
		return err
	}

	var (
		page_size         string                                // Holds the client name, which should be in column 0.
		clientCmpData     = make([]float64, len(columnNames)-1) // 1 less because of the client column.
		clientCmpScanArgs = make([]interface{}, len(columnNames))
	)

	clientCmpScanArgs[0] = &page_size
	for i := range clientCmpData {
		clientCmpScanArgs[i+1] = &clientCmpData[i]
	}

	for informationSchemaInnodbCmpRows.Next() {
		if err := informationSchemaInnodbCmpRows.Scan(clientCmpScanArgs...); err != nil {
			return err
		}

		for idx, columnName := range columnNames[1:] {
			switch columnName {
			case "compress_ops":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressOps, prometheus.CounterValue, float64(clientCmpData[idx]), page_size)
			case "compress_ops_ok":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressOpsOk, prometheus.CounterValue, float64(clientCmpData[idx]), page_size)
			case "compress_time":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressTime, prometheus.CounterValue, float64(clientCmpData[idx]), page_size)
			case "uncompress_ops":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpUncompressOps, prometheus.CounterValue, float64(clientCmpData[idx]), page_size)
			case "uncompress_time":
				ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpUncompressTime, prometheus.CounterValue, float64(clientCmpData[idx]), page_size)
			}
		}
	}

	return nil
}
