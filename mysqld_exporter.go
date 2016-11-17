package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/ini.v1"

	"github.com/prometheus/mysqld_exporter/collector"
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
	slowLogFilter = flag.Bool(
		"log_slow_filter", false,
		"Add a log_slow_filter to avoid exessive MySQL slow logging.  NOTE: Not supported by Oracle MySQL.",
	)
	collectProcesslist = flag.Bool(
		"collect.info_schema.processlist", false,
		"Collect current thread state counts from the information_schema.processlist",
	)
	collectTableSchema = flag.Bool(
		"collect.info_schema.tables", true,
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
		"collect.global_status", true,
		"Collect from SHOW GLOBAL STATUS",
	)
	collectGlobalVariables = flag.Bool(
		"collect.global_variables", true,
		"Collect from SHOW GLOBAL VARIABLES",
	)
	collectSlaveStatus = flag.Bool(
		"collect.slave_status", true,
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
	upQuery              = `SELECT 1`
)

// landingPage contains the HTML served at '/'.
// TODO: Make this nicer and more informative.
var landingPage = []byte(`<html>
<head><title>MySQLd exporter</title></head>
<body>
<h1>MySQLd exporter</h1>
<p><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>
`)

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn             string
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
	scrapeErrors    *prometheus.CounterVec
	mysqldUp        prometheus.Gauge
}

// NewExporter returns a new MySQL exporter for the provided DSN.
func NewExporter(dsn string) *Exporter {
	return &Exporter{
		dsn: dsn,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from MySQL.",
		}),
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

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
	ch <- e.mysqldUp
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	var err error
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
		if err == nil {
			e.error.Set(0)
		} else {
			e.error.Set(1)
		}
	}(time.Now())

	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	isUpRows, err := db.Query(upQuery)
	if err != nil {
		log.Errorln("Error pinging mysqld:", err)
		e.mysqldUp.Set(0)
		return
	}
	isUpRows.Close()

	e.mysqldUp.Set(1)

	if *slowLogFilter {
		sessionSettingsRows, err := db.Query(sessionSettingsQuery)
		if err != nil {
			log.Errorln("Error setting log_slow_filter:", err)
			return
		}
		sessionSettingsRows.Close()
	}

	if *collectGlobalStatus {
		if err = collector.ScrapeGlobalStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.global_status:", err)
			e.scrapeErrors.WithLabelValues("collect.global_status").Inc()
		}
	}
	if *collectGlobalVariables {
		if err = collector.ScrapeGlobalVariables(db, ch); err != nil {
			log.Errorln("Error scraping for collect.global_variables:", err)
			e.scrapeErrors.WithLabelValues("collect.global_variables").Inc()
		}
	}
	if *collectSlaveStatus {
		if err = collector.ScrapeSlaveStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.slave_status:", err)
			e.scrapeErrors.WithLabelValues("collect.slave_status").Inc()
		}
	}
	if *collectProcesslist {
		if err = collector.ScrapeProcesslist(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.processlist:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.processlist").Inc()
		}
	}
	if *collectTableSchema {
		if err = collector.ScrapeTableSchema(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.tables:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tables").Inc()
		}
	}
	if *collectInnodbTablespaces {
		if err = collector.ScrapeInfoSchemaInnodbTablespaces(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.innodb_sys_tablespaces:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.innodb_sys_tablespaces").Inc()
		}
	}
	if *innodbMetrics {
		if err = collector.ScrapeInnodbMetrics(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.innodb_metrics:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.innodb_metrics").Inc()
		}
	}
	if *collectAutoIncrementColumns {
		if err = collector.ScrapeAutoIncrementColumns(db, ch); err != nil {
			log.Errorln("Error scraping for collect.auto_increment.columns:", err)
			e.scrapeErrors.WithLabelValues("collect.auto_increment.columns").Inc()
		}
	}
	if *collectBinlogSize {
		if err = collector.ScrapeBinlogSize(db, ch); err != nil {
			log.Errorln("Error scraping for collect.binlog_size:", err)
			e.scrapeErrors.WithLabelValues("collect.binlog_size").Inc()
		}
	}
	if *collectPerfTableIOWaits {
		if err = collector.ScrapePerfTableIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tableiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tableiowaits").Inc()
		}
	}
	if *collectPerfIndexIOWaits {
		if err = collector.ScrapePerfIndexIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.indexiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.indexiowaits").Inc()
		}
	}
	if *collectPerfTableLockWaits {
		if err = collector.ScrapePerfTableLockWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tablelocks:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tablelocks").Inc()
		}
	}
	if *collectPerfEventsStatements {
		if err = collector.ScrapePerfEventsStatements(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventsstatements:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventsstatements").Inc()
		}
	}
	if *collectPerfEventsWaits {
		if err = collector.ScrapePerfEventsWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventswaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventswaits").Inc()
		}
	}
	if *collectPerfFileEvents {
		if err = collector.ScrapePerfFileEvents(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.file_events:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.file_events").Inc()
		}
	}
	if *collectUserStat {
		if err = collector.ScrapeUserStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.userstats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.userstats").Inc()
		}
	}
	if *collectClientStat {
		if err = collector.ScrapeClientStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.clientstats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.clientstats").Inc()
		}
	}
	if *collectTableStat {
		if err = collector.ScrapeTableStat(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.tablestats:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tablestats").Inc()
		}
	}
	if *collectQueryResponseTime {
		if err = collector.ScrapeQueryResponseTime(db, ch); err != nil {
			log.Errorln("Error scraping for collect.info_schema.query_response_time:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.query_response_time").Inc()
		}
	}
	if *collectEngineTokudbStatus {
		if err = collector.ScrapeEngineTokudbStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.engine_tokudb_status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_tokudb_status").Inc()
		}
	}
	if *collectEngineInnodbStatus {
		if err = collector.ScrapeEngineInnodbStatus(db, ch); err != nil {
			log.Errorln("Error scraping for collect.engine_innodb_status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_innodb_status").Inc()
		}
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

	exporter := NewExporter(dsn)
	prometheus.MustRegister(exporter)

	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
