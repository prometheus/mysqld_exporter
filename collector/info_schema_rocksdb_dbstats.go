// Scrape `information_schema.rocksdb_dbstats`.

package collector

import (
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaRocksDBDBStatsQuery = `SELECT stat_type, value FROM information_schema.rocksdb_dbstats`

// ScrapeRocksDBDBStats collects from `information_schema.rocksdb_dbstats`.
func ScrapeRocksDBDBStats(db *sql.DB, ch chan<- prometheus.Metric) error {
	rows, err := db.Query(infoSchemaRocksDBDBStatsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var typeCol string
	var valueCol int64
	for rows.Next() {
		if err = rows.Scan(&typeCol, &valueCol); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			newDesc("rocksdb_dbstats", strings.ToLower(typeCol), typeCol),
			prometheus.UntypedValue,
			float64(valueCol),
		)
	}
	return rows.Err()
}
