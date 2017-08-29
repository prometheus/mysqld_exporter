package main

import (
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"

	"github.com/percona/mysqld_exporter/collector"
)

var (
	showVersion = flag.Bool(
		"version", false,
		"Print version information.",
	)
	listenAddress = flag.String(
		"web.listen-address", ":9104",
		"Address to listen on for web interface and telemetry.",
	)
	metricPath = flag.String(
		"web.telemetry-path", "/metrics",
		"Path under which to expose metrics.",
	)
	configMycnf = flag.String(
		"config.my-cnf", path.Join(os.Getenv("HOME"), ".my.cnf"),
		"Path to .my.cnf file to read MySQL credentials from.",
	)
	webAuthFile = flag.String(
		"web.auth-file", "",
		"Path to YAML file with server_user, server_password options for http basic auth (overrides HTTP_AUTH env var).",
	)
	sslCertFile = flag.String(
		"web.ssl-cert-file", "",
		"Path to SSL certificate file.",
	)
	sslKeyFile = flag.String(
		"web.ssl-key-file", "",
		"Path to SSL key file.",
	)

	slowLogFilter = flag.Bool(
		"log_slow_filter", false,
		"Add a log_slow_filter to avoid exessive MySQL slow logging.  NOTE: Not supported by Oracle MySQL.",
	)
	collectProcesslist = flag.Bool(
		"collect.info_schema.processlist", false,
		"Collect current thread state counts from the information_schema.processlist",
	)
	collectTableSchema = flag.Bool(
		"collect.info_schema.tables", false,
		"Collect metrics from information_schema.tables",
	)
	collectInnodbTablespaces = flag.Bool(
		"collect.info_schema.innodb_tablespaces", false,
		"Collect metrics from information_schema.innodb_sys_tablespaces",
	)
	innodbMetrics = flag.Bool(
		"collect.info_schema.innodb_metrics", false,
		"Collect metrics from information_schema.innodb_metrics",
	)
	collectGlobalStatus = flag.Bool(
		"collect.global_status", false,
		"Collect from SHOW GLOBAL STATUS",
	)
	collectGlobalVariables = flag.Bool(
		"collect.global_variables", false,
		"Collect from SHOW GLOBAL VARIABLES",
	)
	collectSlaveStatus = flag.Bool(
		"collect.slave_status", false,
		"Collect from SHOW SLAVE STATUS",
	)
	collectAutoIncrementColumns = flag.Bool(
		"collect.auto_increment.columns", false,
		"Collect auto_increment columns and max values from information_schema",
	)
	collectBinlogSize = flag.Bool(
		"collect.binlog_size", false,
		"Collect the current size of all registered binlog files",
	)
	collectPerfTableIOWaits = flag.Bool(
		"collect.perf_schema.tableiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_table",
	)
	collectPerfIndexIOWaits = flag.Bool(
		"collect.perf_schema.indexiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_index_usage",
	)
	collectPerfTableLockWaits = flag.Bool(
		"collect.perf_schema.tablelocks", false,
		"Collect metrics from performance_schema.table_lock_waits_summary_by_table",
	)
	collectPerfEventsStatements = flag.Bool(
		"collect.perf_schema.eventsstatements", false,
		"Collect metrics from performance_schema.events_statements_summary_by_digest",
	)
	collectPerfEventsWaits = flag.Bool(
		"collect.perf_schema.eventswaits", false,
		"Collect metrics from performance_schema.events_waits_summary_global_by_event_name",
	)
	collectPerfFileEvents = flag.Bool(
		"collect.perf_schema.file_events", false,
		"Collect metrics from performance_schema.file_summary_by_event_name",
	)
	collectPerfFileInstances = flag.Bool(
		"collect.perf_schema.file_instances", false,
		"Collect metrics from performance_schema.file_summary_by_instance",
	)
	collectUserStat = flag.Bool("collect.info_schema.userstats", false,
		"If running with userstat=1, set to true to collect user statistics",
	)
	collectClientStat = flag.Bool("collect.info_schema.clientstats", false,
		"If running with userstat=1, set to true to collect client statistics",
	)
	collectTableStat = flag.Bool("collect.info_schema.tablestats", false,
		"If running with userstat=1, set to true to collect table statistics",
	)
	collectQueryResponseTime = flag.Bool("collect.info_schema.query_response_time", false,
		"Collect query response time distribution if query_response_time_stats is ON.",
	)
	collectEngineTokudbStatus = flag.Bool("collect.engine_tokudb_status", false,
		"Collect from SHOW ENGINE TOKUDB STATUS",
	)
	collectEngineInnodbStatus = flag.Bool("collect.engine_innodb_status", false,
		"Collect from SHOW ENGINE INNODB STATUS",
	)
	collectEngineRocksDBStatus = flag.Bool("collect.engine_rocksdb_status", false,
		"Collect from SHOW ENGINE ROCKSDB STATUS",
	)
	collectHeartbeat = flag.Bool(
		"collect.heartbeat", false,
		"Collect from heartbeat",
	)
	collectHeartbeatDatabase = flag.String(
		"collect.heartbeat.database", "heartbeat",
		"Database from where to collect heartbeat data",
	)
	collectHeartbeatTable = flag.String(
		"collect.heartbeat.table", "heartbeat",
		"Table from where to collect heartbeat data",
	)
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "mysql"
	// Subsystem(s).
	exporter = "exporter"
)

// SQL Queries.
const (
	sessionSettingsQuery = `SET SESSION log_slow_filter = 'tmp_table_on_disk,filesort_on_disk'`
	versionQuery         = `SELECT @@version`
)

// landingPage contains the HTML served at '/'.
var landingPage = []byte(`<html>
<head><title>MySQLd 3-in-1 exporter</title></head>
<body>
<h1>MySQL 3-in-1 exporter</h1>
<li><a href="/` + *metricPath + `-hr">high-res metrics</a></li>
<li><a href="/` + *metricPath + `-mr">medium-res metrics</a></li>
<li><a href="/` + *metricPath + `-lr">low-res metrics</a></li>
</body>
</html>
`)

// Metric descriptors.
var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

type webAuth struct {
	User     string `yaml:"server_user,omitempty"`
	Password string `yaml:"server_password,omitempty"`
}

type basicAuthHandler struct {
	handler  http.HandlerFunc
	user     string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || password != h.password || user != h.user {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
}

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn          string
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
	mysqldUp     prometheus.Gauge
}

// ExporterMr collects MySQL metrics. It implements prometheus.Collector.
type ExporterMr struct {
	dsn          string
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
}

// ExporterLr collects MySQL metrics. It implements prometheus.Collector.
type ExporterLr struct {
	dsn          string
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
}

// NewExporter returns a new MySQL exporter for the provided DSN.
func NewExporter(dsn string) *Exporter {
	exporter := "exporter_hr"
	return &Exporter{
		dsn: dsn,
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

// NewExporterMr returns a new MySQL exporter for the provided DSN.
func NewExporterMr(dsn string) *ExporterMr {
	exporter := "exporter_mr"
	return &ExporterMr{
		dsn: dsn,
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
	}
}

// NewExporterLr returns a new MySQL exporter for the provided DSN.
func NewExporterLr(dsn string) *ExporterLr {
	exporter := "exporter_lr"
	return &ExporterLr{
		dsn: dsn,
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

// Describe implements prometheus.Collector.
func (e *ExporterMr) Describe(ch chan<- *prometheus.Desc) {
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

// Describe implements prometheus.Collector.
func (e *ExporterLr) Describe(ch chan<- *prometheus.Desc) {
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

// Collect implements prometheus.Collector.
func (e *ExporterMr) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
}

// Collect implements prometheus.Collector.
func (e *ExporterLr) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Errorln("Error pinging mysqld:", err)
		e.mysqldUp.Set(0)
		return
	}
	e.mysqldUp.Set(1)
	versionNum := getMySQLVersion(db)

	if *slowLogFilter {
		sessionSettingsRows, err := db.Query(sessionSettingsQuery)
		if err != nil {
			log.Errorln("Error setting log_slow_filter:", err)
			return
		}
		sessionSettingsRows.Close()
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	if *collectGlobalStatus {
		scrapeTime = time.Now()
		if err = collector.ScrapeGlobalStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.global_status:", err)
			e.scrapeErrors.WithLabelValues("collect.global_status").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.global_status")
	}

	if *innodbMetrics && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapeInnodbMetrics(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.innodb_metrics:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.innodb_metrics").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.innodb_metrics")
	}
}

func (e *ExporterMr) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Errorln("Error pinging mysqld:", err)
		return
	}
	versionNum := getMySQLVersion(db)

	if *slowLogFilter {
		sessionSettingsRows, err := db.Query(sessionSettingsQuery)
		if err != nil {
			log.Errorln("Error setting log_slow_filter:", err)
			return
		}
		sessionSettingsRows.Close()
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	if *collectSlaveStatus {
		scrapeTime = time.Now()
		if err = collector.ScrapeSlaveStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.slave_status:", err)
			e.scrapeErrors.WithLabelValues("collect.slave_status").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.slave_status")
	}
	if *collectProcesslist {
		scrapeTime = time.Now()
		if err = collector.ScrapeProcesslist(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.processlist:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.processlist").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.processlist")
	}
	if *collectPerfEventsWaits && versionNum >= 5.5 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfEventsWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventswaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventswaits").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.eventswaits")
	}
	if *collectPerfFileEvents && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfFileEvents(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.file_events:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.file_events").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.file_events")
	}
	if *collectPerfTableLockWaits && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfTableLockWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tablelocks:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tablelocks").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.tablelocks")
	}
	if *collectQueryResponseTime && versionNum >= 5.5 {
		scrapeTime = time.Now()
		if err = collector.ScrapeQueryResponseTime(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.query_response_time:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.query_response_time").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.query_response_time")
	}
	if *collectEngineInnodbStatus {
		scrapeTime = time.Now()
		if err = collector.ScrapeEngineInnodbStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.engine_innodb_status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_innodb_status").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.engine_innodb_status")
	}
	if *collectEngineRocksDBStatus {
		scrapeTime = time.Now()
		if err = collector.ScrapeEngineRocksDBStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.engine_rocksdb_status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_rocksdb_status").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.engine_rocksdb_status")
	}
}

func (e *ExporterLr) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error

	scrapeTime := time.Now()
	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Errorln("Error pinging mysqld:", err)
		return
	}
	versionNum := getMySQLVersion(db)

	if *slowLogFilter {
		sessionSettingsRows, err := db.Query(sessionSettingsQuery)
		if err != nil {
			log.Errorln("Error setting log_slow_filter:", err)
			return
		}
		sessionSettingsRows.Close()
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	if *collectGlobalVariables {
		scrapeTime = time.Now()
		if err = collector.ScrapeGlobalVariables(db, ch); err != nil {
			log.Errorln("Error scraping for collect.global_variables:", err)
			e.scrapeErrors.WithLabelValues("collect.global_variables").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.global_variables")
	}
	if *collectTableSchema {
		scrapeTime = time.Now()
		if err = collector.ScrapeTableSchema(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.tables:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tables").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.tables")
	}
	if *collectAutoIncrementColumns {
		scrapeTime = time.Now()
		if err = collector.ScrapeAutoIncrementColumns(db, ch); err != nil {
			log.Errorln("Error scraping for collect.auto_increment.columns:", err)
			e.scrapeErrors.WithLabelValues("collect.auto_increment.columns").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.auto_increment.columns")
	}
	if *collectBinlogSize {
		scrapeTime = time.Now()
		if err = collector.ScrapeBinlogSize(db, ch); err != nil {
			log.Errorln("Error scraping for collect.binlog_size:", err)
			e.scrapeErrors.WithLabelValues("collect.binlog_size").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.binlog_size")
	}
	if *collectPerfTableIOWaits && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfTableIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tableiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tableiowaits").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.tableiowaits")
	}
	if *collectPerfIndexIOWaits && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfIndexIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.indexiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.indexiowaits").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.indexiowaits")
	}
	if *collectPerfFileInstances && versionNum >= 5.5 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfFileInstances(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.file_instances:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.file_instances").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.file_instances")
	}
	if *collectUserStat {
		scrapeTime = time.Now()
		if err = collector.ScrapeUserStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.userstats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.userstats").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.userstats")
	}
	if *collectTableStat {
		scrapeTime = time.Now()
		if err = collector.ScrapeTableStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.tablestats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tablestats").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.tablestats")
	}
	if *collectPerfEventsStatements && versionNum >= 5.6 {
		scrapeTime = time.Now()
		if err = collector.ScrapePerfEventsStatements(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventsstatements:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventsstatements").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.perf_schema.eventsstatements")
	}
	if *collectClientStat && versionNum >= 5.5 {
		scrapeTime = time.Now()
		if err = collector.ScrapeClientStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.clientstats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.clientstats").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.clientstats")
	}
	if *collectInnodbTablespaces && versionNum >= 5.7 {
		scrapeTime = time.Now()
		if err = collector.ScrapeInfoSchemaInnodbTablespaces(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.innodb_sys_tablespaces:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.innodb_sys_tablespaces").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.info_schema.innodb_sys_tablespaces")
	}
	if *collectEngineTokudbStatus && versionNum >= 5.7 {
		scrapeTime = time.Now()
		if err = collector.ScrapeEngineTokudbStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.engine_tokudb_status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_tokudb_status").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.engine_tokudb_status")
	}
	if *collectHeartbeat && versionNum >= 5.1 {
		scrapeTime = time.Now()
		if err = collector.ScrapeHeartbeat(db, ch, collectHeartbeatDatabase, collectHeartbeatTable); err != nil {
			log.Errorln("Error scraping for collect.heartbeat:", err)
			e.scrapeErrors.WithLabelValues("collect.heartbeat").Inc()
		}
		ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.heartbeat")
	}
}

func parseMycnf(config interface{}) (string, error) {
	var dsn string
	cfg, err := ini.Load(config)
	if err != nil {
		return dsn, fmt.Errorf("failed reading ini file: %s", err)
	}
	user := cfg.Section("client").Key("user").String()
	password := cfg.Section("client").Key("password").String()
	if (user == "") || (password == "") {
		return dsn, fmt.Errorf("no user or password specified under [client] in %s", config)
	}
	host := cfg.Section("client").Key("host").MustString("localhost")
	port := cfg.Section("client").Key("port").MustUint(3306)
	socket := cfg.Section("client").Key("socket").String()
	if socket != "" {
		dsn = fmt.Sprintf("%s:%s@unix(%s)/", user, password, socket)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	}
	log.Debugln(dsn)
	return dsn, nil
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

func init() {
	prometheus.MustRegister(version.NewCollector("mysqld_exporter"))
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("mysqld_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting mysqld_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if len(dsn) == 0 {
		var err error
		if dsn, err = parseMycnf(*configMycnf); err != nil {
			log.Fatal(err)
		}
	}

	cfg := &webAuth{}
	httpAuth := os.Getenv("HTTP_AUTH")
	if *webAuthFile != "" {
		bytes, err := ioutil.ReadFile(*webAuthFile)
		if err != nil {
			log.Fatal("Cannot read auth file: ", err)
		}
		if err := yaml.Unmarshal(bytes, cfg); err != nil {
			log.Fatal("Cannot parse auth file: ", err)
		}
	} else if httpAuth != "" {
		data := strings.SplitN(httpAuth, ":", 2)
		if len(data) != 2 || data[0] == "" || data[1] == "" {
			log.Fatal("HTTP_AUTH should be formatted as user:password")
		}
		cfg.User = data[0]
		cfg.Password = data[1]
	}
	if cfg.User != "" && cfg.Password != "" {
		log.Infoln("HTTP basic authentication is enabled")
	}

	if *sslCertFile != "" && *sslKeyFile == "" || *sslCertFile == "" && *sslKeyFile != "" {
		log.Fatal("One of the flags -web.ssl-cert or -web.ssl-key is missed to enable HTTPS/TLS")
	}
	ssl := false
	if *sslCertFile != "" && *sslKeyFile != "" {
		if _, err := os.Stat(*sslCertFile); os.IsNotExist(err) {
			log.Fatal("SSL certificate file does not exist: ", *sslCertFile)
		}
		if _, err := os.Stat(*sslKeyFile); os.IsNotExist(err) {
			log.Fatal("SSL key file does not exist: ", *sslKeyFile)
		}
		ssl = true
		log.Infoln("HTTPS/TLS is enabled")
	}

	log.Infoln("Listening on", *listenAddress)
	if ssl {
		// https
		mux := http.NewServeMux()

		reg := prometheus.NewRegistry()
		reg.MustRegister(NewExporter(dsn))
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		mux.Handle(*metricPath+"-hr", handler)

		reg = prometheus.NewRegistry()
		reg.MustRegister(NewExporterMr(dsn))
		handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		mux.Handle(*metricPath+"-mr", handler)

		reg = prometheus.NewRegistry()
		reg.MustRegister(NewExporterLr(dsn))
		handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		mux.Handle(*metricPath+"-lr", handler)

		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			w.Write(landingPage)
		})
		tlsCfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
		srv := &http.Server{
			Addr:         *listenAddress,
			Handler:      mux,
			TLSConfig:    tlsCfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}
		log.Fatal(srv.ListenAndServeTLS(*sslCertFile, *sslKeyFile))
	} else {
		// http
		reg := prometheus.NewRegistry()
		reg.MustRegister(NewExporter(dsn))
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		http.Handle(*metricPath+"-hr", handler)

		reg = prometheus.NewRegistry()
		reg.MustRegister(NewExporterMr(dsn))
		handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		http.Handle(*metricPath+"-mr", handler)

		reg = prometheus.NewRegistry()
		reg.MustRegister(NewExporterLr(dsn))
		handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			handler = &basicAuthHandler{handler: handler.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		http.Handle(*metricPath+"-lr", handler)

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage)
		})

		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}
}
