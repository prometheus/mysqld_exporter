// Scrape custom queries

package collector

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/go-kit/kit/log"
)

// ScrapeCustomQuery collects the metrics from custom queries.
type ScrapeCustomQuery struct{}


// Name of the Scraper.
func (scq ScrapeCustomQuery) Name() string {
	return fmt.Sprintf("custom_query")
}

// Help returns additional information about Scraper.
func (scq ScrapeCustomQuery) Help() string {
	return fmt.Sprintf("Collect the metrics from custom queries.")
}

// Version of MySQL from which scraper is available.
func (scq ScrapeCustomQuery) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (scq ScrapeCustomQuery) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	return nil
}