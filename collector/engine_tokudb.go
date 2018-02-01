// Scrape `SHOW ENGINE TOKUDB STATUS`.

package collector

import (
	"database/sql"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem.
	tokudb = "engine_tokudb"
	// Query.
	engineTokudbStatusQuery = `SHOW ENGINE TOKUDB STATUS`
)

// ScrapeEngineTokudbStatus scrapes from `SHOW ENGINE TOKUDB STATUS`.
type ScrapeEngineTokudbStatus struct{}

// Name of the Scraper. Should be unique.
func (ScrapeEngineTokudbStatus) Name() string {
	return "engine_tokudb_status"
}

// Help describes the role of the Scraper.
func (ScrapeEngineTokudbStatus) Help() string {
	return "Collect from SHOW ENGINE TOKUDB STATUS"
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeEngineTokudbStatus) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	tokudbRows, err := db.Query(engineTokudbStatusQuery)
	if err != nil {
		return err
	}
	defer tokudbRows.Close()

	var temp, key string
	var val sql.RawBytes

	for tokudbRows.Next() {
		if err := tokudbRows.Scan(&temp, &key, &val); err != nil {
			return err
		}
		key = strings.ToLower(key)
		if floatVal, ok := parseStatus(val); ok {
			ch <- prometheus.MustNewConstMetric(
				newDesc(tokudb, sanitizeTokudbMetric(key), "Generic metric from SHOW ENGINE TOKUDB STATUS."),
				prometheus.UntypedValue,
				floatVal,
			)
		}
	}
	return nil
}

func sanitizeTokudbMetric(metricName string) string {
	replacements := map[string]string{
		">": "",
		",": "",
		":": "",
		"(": "",
		")": "",
		" ": "_",
		"-": "_",
		"+": "and",
		"/": "and",
	}
	for r := range replacements {
		metricName = strings.Replace(metricName, r, replacements[r], -1)
	}
	return metricName
}
