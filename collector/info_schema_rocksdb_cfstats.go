// Scrape `information_schema.rocksdb_cfstats`.

package collector

import (
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaRocksDBCFStatsQuery = `SELECT cf_name, stat_type, value FROM information_schema.rocksdb_cfstats`

// ScrapeRocksDBCFStats collects from `information_schema.rocksdb_cfstats`.
func ScrapeRocksDBCFStats(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(infoSchemaRocksDBCFStatsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var nameCol, typeCol string
	var valueCol int64
	for rows.Next() {
		if err = rows.Scan(&nameCol, &typeCol, &valueCol); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "rocksdb_cfstats", strings.ToLower(typeCol)),
				typeCol, []string{"name"}, nil,
			),
			prometheus.UntypedValue,
			float64(valueCol),
			nameCol,
		)
	}
	return rows.Err()
}
