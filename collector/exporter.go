package collector

import (
	"database/sql"
	"fmt"
	"strings"
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

// Metric descriptors.
var (
	exporterLockTimeout = kingpin.Flag(
		"exporter.lock_wait_timeout",
		"Set a lock_wait_timeout on the connection to avoid long metadata locking.",
	).Default("2").Int()
	slowLogFilter = kingpin.Flag(
		"exporter.log_slow_filter",
		"Add a log_slow_filter to avoid slow query logging of scrapes. NOTE: Not supported by Oracle MySQL.",
	).Default("false").Bool()

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

// Collect defines which metrics we should collect
type Collect struct {
	Processlist          bool
	TableSchema          bool
	InnodbTablespaces    bool
	InnodbMetrics        bool
	GlobalStatus         bool
	GlobalVariables      bool
	SlaveStatus          bool
	AutoIncrementColumns bool
	BinlogSize           bool
	PerfTableIOWaits     bool
	PerfIndexIOWaits     bool
	PerfTableLockWaits   bool
	PerfEventsStatements bool
	PerfEventsWaits      bool
	PerfFileEvents       bool
	PerfFileInstances    bool
	UserStat             bool
	ClientStat           bool
	TableStat            bool
	QueryResponseTime    bool
	EngineTokudbStatus   bool
	EngineInnodbStatus   bool
	Heartbeat            bool
	HeartbeatDatabase    string
	HeartbeatTable       string
}

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn          string
	collect      Collect
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
	mysqldUp     prometheus.Gauge
}

// New returns a new MySQL exporter for the provided DSN.
func New(dsn string, collect Collect) *Exporter {
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
		dsn:     dsn,
		collect: collect,
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

type scrapeFunc func(*sql.DB, chan<- prometheus.Metric) error

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()

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
		e.error.Set(1)
		return
	}
	isUpRows.Close()

	e.mysqldUp.Set(1)

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	scrapeTargets := []struct {
		enabled bool
		label   string
		fn      scrapeFunc
	}{
		// "log_slow_filter" needs to be the first, since it changes the session.
		// TODO: Set via DSN parameter instead? See https://github.com/go-sql-driver/mysql/#system-variables
		{e.collect.SlowLogFilter, "log_slow_filter", func(db *sql.DB, ch chan<- prometheus.Metric) error {
			sessionSettingsRows, err := db.Query(sessionSettingsQuery)
			if err != nil {
				return err
			}

			return sessionSettingsRows.Close()
		}},
		{e.collect.GlobalStatus, "collect.global_status", ScrapeGlobalStatus},
		{e.collect.GlobalVariables, "collect.global_variables", ScrapeGlobalVariables},
		{e.collect.SlaveStatus, "collect.slave_status", ScrapeSlaveStatus},
		{e.collect.Processlist, "collect.info_schema.processlist", ScrapeProcesslist},
		{e.collect.TableSchema, "collect.info_schema.tables", ScrapeTableSchema},
		{e.collect.InnodbTablespaces, "collect.info_schema.innodb_sys_tablespaces", ScrapeInfoSchemaInnodbTablespaces},
		{e.collect.InnodbMetrics, "collect.info_schema.innodb_metrics", ScrapeInnodbMetrics},
		{e.collect.AutoIncrementColumns, "collect.auto_increment.columns", ScrapeAutoIncrementColumns},
		{e.collect.BinlogSize, "collect.binlog_size", ScrapeBinlogSize},
		{e.collect.PerfTableIOWaits, "collect.perf_schema.tableiowaits", ScrapePerfTableIOWaits},
		{e.collect.PerfIndexIOWaits, "collect.perf_schema.indexiowaits", ScrapePerfIndexIOWaits},
		{e.collect.PerfTableLockWaits, "collect.perf_schema.tablelocks", ScrapePerfTableLockWaits},
		{e.collect.PerfEventsStatements, "collect.perf_schema.eventsstatements", ScrapePerfEventsStatements},
		{e.collect.PerfEventsWaits, "collect.perf_schema.eventswaits", ScrapePerfEventsWaits},
		{e.collect.PerfFileEvents, "collect.perf_schema.file_events", ScrapePerfFileEvents},
		{e.collect.PerfFileInstances, "collect.perf_schema.file_instances", ScrapePerfFileInstances},
		{e.collect.UserStat, "collect.info_schema.userstats", ScrapeUserStat},
		{e.collect.ClientStat, "collect.info_schema.clientstats", ScrapeClientStat},
		{e.collect.TableStat, "collect.info_schema.tablestats", ScrapeTableStat},
		{e.collect.QueryResponseTime, "collect.info_schema.query_response_time", ScrapeQueryResponseTime},
		{e.collect.EngineTokudbStatus, "collect.engine_tokudb_status", ScrapeEngineTokudbStatus},
		{e.collect.EngineInnodbStatus, "collect.engine_innodb_status", ScrapeEngineInnodbStatus},
		{e.collect.Heartbeat, "collect.heartbeat", func(db *sql.DB, ch chan<- prometheus.Metric) error {
			return ScrapeHeartbeat(db, ch, e.collect.HeartbeatDatabase, e.collect.HeartbeatTable)
		}},
	}

	for _, st := range scrapeTargets {
		if !st.enabled {
			continue
		}

		e.scrapeOne(db, ch, st.label, st.fn)
	}
}

func (e *Exporter) scrapeOne(db *sql.DB, ch chan<- prometheus.Metric, l string, f scrapeFunc) {
	defer func(s time.Time) {
		ch <- prometheus.MustNewConstMetric(
			scrapeDurationDesc,
			prometheus.GaugeValue,
			time.Since(s).Seconds(),
			l,
		)
	}(time.Now())

	if err := f(db, ch); err != nil {
		log.Errorf("Error scraping for %s: %s", l, err)
		e.scrapeErrors.WithLabelValues(l).Inc()
		e.error.Set(1)
	}
}
