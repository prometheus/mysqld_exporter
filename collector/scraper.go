package collector

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

type Scraper interface {
	Scrape(db *sql.DB, ch chan<- prometheus.Metric) error
	Name() string
	Help() string
}
