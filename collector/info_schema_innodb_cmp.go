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

// Metric descriptors.
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
type ScrapeInnodbCmp struct{}

// Name of the Scraper. Should be unique.
func (ScrapeInnodbCmp) Name() string {
	return informationSchema + ".innodb_cmp"
}

// Help describes the role of the Scraper.
func (ScrapeInnodbCmp) Help() string {
	return "Collect metrics from information_schema.innodb_cmp"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeInnodbCmp) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {

	informationSchemaInnodbCmpRows, err := db.Query(innodbCmpQuery)
	if err != nil {
		return err
	}
	defer informationSchemaInnodbCmpRows.Close()

	var (
		page_size                                                                     string
		compress_ops, compress_ops_ok, compress_time, uncompress_ops, uncompress_time float64
	)

	for informationSchemaInnodbCmpRows.Next() {

		if err := informationSchemaInnodbCmpRows.Scan(
			&page_size, &compress_ops, &compress_ops_ok, &compress_time, &uncompress_ops, &uncompress_time,
		); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressOps, prometheus.CounterValue, compress_ops, page_size)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressOpsOk, prometheus.CounterValue, compress_ops_ok, page_size)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpCompressTime, prometheus.CounterValue, compress_time, page_size)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpUncompressOps, prometheus.CounterValue, uncompress_ops, page_size)
		ch <- prometheus.MustNewConstMetric(infoSchemaInnodbCmpUncompressTime, prometheus.CounterValue, uncompress_time, page_size)

	}

	return nil
}
