// Scrape `information_schema.table_statistics`.

package collector

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const tableStatQuery = `
		SELECT
		  TABLE_SCHEMA,
		  TABLE_NAME,
		  ROWS_READ,
		  ROWS_CHANGED,
		  ROWS_CHANGED_X_INDEXES
		  FROM information_schema.table_statistics
		`

// Metric descriptors.
var (
	infoSchemaTableStatsRowsReadDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_read_total"),
		"The number of rows read from the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsRowsChangedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_total"),
		"The number of rows changed in the table.",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTableStatsRowsChangedXIndexesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_statistics_rows_changed_x_indexes_total"),
		"The number of rows changed in the table, multiplied by the number of indexes changed.",
		[]string{"schema", "table"}, nil,
	)
)

// ScrapeTableStat collects from `information_schema.table_statistics`.
type ScrapeTableStat struct{}

// Name of the Scraper. Should be unique.
func (ScrapeTableStat) Name() string {
	return "info_schema.tablestats"
}

// Help describes the role of the Scraper.
func (ScrapeTableStat) Help() string {
	return "If running with userstat=1, set to true to collect table statistics"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeTableStat) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	var varName, varVal string
	err := db.QueryRow(userstatCheckQuery).Scan(&varName, &varVal)
	if err != nil {
		log.Debugln("Detailed table stats are not available.")
		return nil
	}
	if varVal == "OFF" {
		log.Debugf("MySQL @@%s is OFF.", varName)
		return nil
	}

	informationSchemaTableStatisticsRows, err := db.Query(tableStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaTableStatisticsRows.Close()

	var (
		tableSchema         string
		tableName           string
		rowsRead            uint64
		rowsChanged         uint64
		rowsChangedXIndexes uint64
	)

	for informationSchemaTableStatisticsRows.Next() {
		err = informationSchemaTableStatisticsRows.Scan(
			&tableSchema,
			&tableName,
			&rowsRead,
			&rowsChanged,
			&rowsChangedXIndexes,
		)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsReadDesc, prometheus.CounterValue, float64(rowsRead),
			tableSchema, tableName,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsChangedDesc, prometheus.CounterValue, float64(rowsChanged),
			tableSchema, tableName,
		)
		ch <- prometheus.MustNewConstMetric(
			infoSchemaTableStatsRowsChangedXIndexesDesc, prometheus.CounterValue, float64(rowsChangedXIndexes),
			tableSchema, tableName,
		)
	}
	return nil
}
