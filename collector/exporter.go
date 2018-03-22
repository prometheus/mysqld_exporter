package collector

import (
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

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	db       *sql.DB
	scrapers []Scraper
	stats    *Stats
	mysqldUp prometheus.Gauge
}

// New returns a new MySQL exporter for the provided DSN.
func New(db *sql.DB, scrapers []Scraper, stats *Stats) *Exporter {
	return &Exporter{
		db:       db,
		scrapers: scrapers,
		stats:    stats,
		mysqldUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the MySQL server is up.",
		}),
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// from MySQL. So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the MySQL DB. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored MySQL instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.stats.TotalScrapes
	ch <- e.stats.Error
	e.stats.ScrapeErrors.Collect(ch)
	ch <- e.mysqldUp
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.stats.TotalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	if err = e.db.Ping(); err != nil {
		log.Errorln("Error pinging mysqld:", err)
		e.mysqldUp.Set(0)
		e.stats.Error.Set(1)
		return
	}
	e.mysqldUp.Set(1)
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	versionNum := getMySQLVersion(e.db)
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
			if err := scraper.Scrape(e.db, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.stats.ScrapeErrors.WithLabelValues(label).Inc()
				e.stats.Error.Set(1)
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
}

func getMySQLVersion(db *sql.DB) float64 {
	var (
		versionStr string
		versionNum float64
	)
	err := db.QueryRow(versionQuery).Scan(&versionStr)
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

type Stats struct {
	TotalScrapes prometheus.Counter
	ScrapeErrors *prometheus.CounterVec
	Error        prometheus.Gauge
}

func NewStats(resolution string) *Stats {
	subsystem := exporter
	if resolution != "" {
		subsystem = exporter + "_" + resolution
	}
	return &Stats{
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
	}
}
