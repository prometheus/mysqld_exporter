package collector

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// Metric name parts.
const (
	// Subsystem(s).
	exporter = "exporter"
)

// SQL Queries.
const (
	versionQuery = `SELECT @@version`
)

// Metric descriptors.
var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

// Verify if Exporter implements prometheus.Collector
var _ prometheus.Collector = (*Exporter)(nil)

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	ctx      context.Context
	db       *sql.DB
	scrapers []Scraper
	metrics  Metrics
}

// New returns a new MySQL exporter for the provided DSN.
func New(ctx context.Context, db *sql.DB, metrics Metrics, scrapers []Scraper) *Exporter {
	return &Exporter{
		ctx:      ctx,
		db:       db,
		scrapers: scrapers,
		metrics:  metrics,
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.metrics.TotalScrapes.Desc()
	ch <- e.metrics.Error.Desc()
	e.metrics.ScrapeErrors.Describe(ch)
	ch <- e.metrics.MySQLUp.Desc()
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(e.ctx, ch)

	ch <- e.metrics.TotalScrapes
	ch <- e.metrics.Error
	e.metrics.ScrapeErrors.Collect(ch)
	ch <- e.metrics.MySQLUp
}

func (e *Exporter) scrape(ctx context.Context, ch chan<- prometheus.Metric) {
	e.metrics.Error.Set(0)
	e.metrics.TotalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	if err = e.db.PingContext(ctx); err != nil {
		// BUG(arvenil): PMM-2726: When PingContext returns with context deadline exceeded
		// the subsequent call will return `bad connection`.
		// https://github.com/go-sql-driver/mysql/issues/858
		// The PingContext is called second time as a workaround for this issue.
		if err = e.db.PingContext(ctx); err != nil {
			log.Errorln("Error pinging mysqld:", err)
			e.metrics.MySQLUp.Set(0)
			e.metrics.Error.Set(1)
			return
		}
	}
	e.metrics.MySQLUp.Set(1)

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	versionNum := getMySQLVersion(ctx, e.db)
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		if versionNum < scraper.Version() {
			continue
		}
		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			label := "collect." + scraper.Name()
			scrapeTime := time.Now()
			if err := scraper.Scrape(ctx, e.db, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.metrics.ScrapeErrors.WithLabelValues(label).Inc()
				e.metrics.Error.Set(1)
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
}

func getMySQLVersion(ctx context.Context, db *sql.DB) float64 {
	var (
		versionStr string
		versionNum float64
	)
	err := db.QueryRowContext(ctx, versionQuery).Scan(&versionStr)
	if err == nil {
		r, _ := regexp.Compile(`^\d+\.\d+`)
		versionNum, _ = strconv.ParseFloat(r.FindString(versionStr), 64)
	}
	// In case, we can't match/parse the version, let's set it to something big to it matches all the versions.
	if versionNum == 0 {
		versionNum = 999
	}
	return versionNum
}

// Metrics represents exporter metrics which values can be carried between http requests.
type Metrics struct {
	TotalScrapes prometheus.Counter
	ScrapeErrors *prometheus.CounterVec
	Error        prometheus.Gauge
	MySQLUp      prometheus.Gauge
}

// NewMetrics creates new Metrics instance.
func NewMetrics(resolution string) Metrics {
	subsystem := exporter
	if resolution != "" {
		subsystem = exporter + "_" + resolution
	}
	return Metrics{
		TotalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrapes_total",
			Help:      "Total number of times MySQL was scraped for metrics.",
		}),
		ScrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a MySQL.",
		}, []string{"collector"}),
		Error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from MySQL resulted in an error (1 for error, 0 for success).",
		}),
		MySQLUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the MySQL server is up.",
		}),
	}
}
