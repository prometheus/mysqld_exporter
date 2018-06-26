package collector

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Metric name parts.
const (
	// Subsystem(s).
	exporter = "exporter"
)

// SQL Queries.
const (
	// System variable params formatting.
	// See: https://github.com/go-sql-driver/mysql#system-variables
	sessionSettingsParam = `log_slow_filter=%27tmp_table_on_disk,filesort_on_disk%27`
	timeoutParam         = `lock_wait_timeout=%d`

	upQuery = `SELECT 1`
)

// Tunable flags.
var (
	exporterLockTimeout = kingpin.Flag(
		"exporter.lock_wait_timeout",
		"Set a lock_wait_timeout on the connection to avoid long metadata locking.",
	).Default("2").Int()
	slowLogFilter = kingpin.Flag(
		"exporter.log_slow_filter",
		"Add a log_slow_filter to avoid slow query logging of scrapes. NOTE: Not supported by Oracle MySQL.",
	).Default("false").Bool()
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
	dsn          string
	scrapers     []Scraper
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
	mysqldUp     prometheus.Gauge
	mysqlHealth  MysqlHealth
}
type MysqlHealth struct {
	sync.Mutex
	state error
}

func (m *MysqlHealth) Set(state error) {
	m.Lock()
	m.state = state
	m.Unlock()
}

func (m *MysqlHealth) Get() error {
	m.Lock()
	defer m.Unlock()
	return m.state
}

// New returns a new MySQL exporter for the provided DSN.
func New(dsn string, scrapers []Scraper) *Exporter {
	// Setup extra params for the DSN, default to having a lock timeout.
	dsnParams := []string{fmt.Sprintf(timeoutParam, *exporterLockTimeout)}

	if *slowLogFilter {
		dsnParams = append(dsnParams, sessionSettingsParam)
	}

	if strings.Contains(dsn, "?") {
		dsn = dsn + "&"
	} else {
		dsn = dsn + "?"
	}
	dsn += strings.Join(dsnParams, "&")

	return &Exporter{
		dsn:      dsn,
		scrapers: scrapers,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times MySQL was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a MySQL.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from MySQL resulted in an error (1 for error, 0 for success).",
		}),
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

	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
	ch <- e.mysqldUp
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		e.error.Set(1)
		return
	}
	defer db.Close()

	// By design exporter should use maximum one connection per request.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// Set max lifetime for a connection.
	db.SetConnMaxLifetime(1 * time.Minute)

	isUpRows, err := db.Query(upQuery)
	if err != nil {
		log.Errorln("Error pinging mysqld:", err)
		e.mysqldUp.Set(0)

		// Set the error state to err, mysqld is down.
		e.mysqlHealth.Set(err)

		e.error.Set(1)
		return
	}
	isUpRows.Close()

	e.mysqldUp.Set(1)
	// Set the error state to nil, mysqld is up.
	e.mysqlHealth.Set(nil)

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			label := "collect." + scraper.Name()
			scrapeTime := time.Now()
			if err := scraper.Scrape(db, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.scrapeErrors.WithLabelValues(label).Inc()
				e.error.Set(1)
			}
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
}

// MysqlHealth exports the MySQL health state.
func (e *Exporter) MysqlHealth() error {

	return e.mysqlHealth.Get()
}
