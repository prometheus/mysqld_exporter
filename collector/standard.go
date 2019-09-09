package collector

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
)

type standardGo struct {
	c prometheus.Collector
}

func NewStandardGo() Scraper {
	return standardGo{
		c: prometheus.NewGoCollector(),
	}
}

// Name of the Scraper.
func (standardGo) Name() string {
	return "standard.go"
}

// Help returns additional information about Scraper.
func (standardGo) Help() string {
	return "Collect exporter Go process metrics"
}

// Version of MySQL from which scraper is available.
func (standardGo) Version() float64 {
	return 0
}

// Scrape collects data.
func (s standardGo) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	s.c.Collect(ch)
	return nil
}

type standardProcess struct {
	c prometheus.Collector
}

func NewStandardProcess() Scraper {
	return standardProcess{
		c: prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	}
}

// Name of the Scraper.
func (standardProcess) Name() string {
	return "standard.process"
}

// Help returns additional information about Scraper.
func (standardProcess) Help() string {
	return "Collect exporter process metrics"
}

// Version of MySQL from which scraper is available.
func (standardProcess) Version() float64 {
	return 0
}

// Scrape collects data.
func (s standardProcess) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	s.c.Collect(ch)
	return nil
}
