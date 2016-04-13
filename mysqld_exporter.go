package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/ini.v1"

	"github.com/prometheus/mysqld_exporter/collector"
)

var (
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
	perfEventsStatementsLimit = flag.Int(
		"collect.perf_schema.eventsstatements.limit", 250,
		"Limit the number of events statements digests by response time",
	)
	perfEventsStatementsTimeLimit = flag.Int(
		"collect.perf_schema.eventsstatements.timelimit", 86400,
		"Limit how old the 'last_seen' events statements can be, in seconds",
	)
	perfEventsStatementsDigestTextLimit = flag.Int(
		"collect.perf_schema.eventsstatements.digest_text_limit", 120,
		"Maximum length of the normalized statement text",
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
	collectTableStat = flag.Bool("collect.info_schema.tablestats", false,
		"If running with userstat=1, set to true to collect table statistics",
	)
	collectQueryResponseTime = flag.Bool("collect.info_schema.query_response_time", false,
		"Collect query response time distribution if query_response_time_stats is ON.")
	collectEngineTokudbStatus = flag.Bool("collect.engine_tokudb_status", false,
		"Collect from SHOW ENGINE TOKUDB STATUS")
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "mysql"
	// Subsystems.
	exporter          = "exporter"
	performanceSchema = "perf_schema"
)

// Metric SQL Queries.
const (
	sessionSettingsQuery  = `SET SESSION log_slow_filter = 'tmp_table_on_disk,filesort_on_disk'`
	upQuery               = `SELECT 1`
	perfTableIOWaitsQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE,
		  SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
		  FROM performance_schema.table_io_waits_summary_by_table
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
		`
	perfIndexIOWaitsQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME,
		  COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE,
		  SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
		  FROM performance_schema.table_io_waits_summary_by_index_usage
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
		`
	perfTableLockWaitsQuery = `
		SELECT
		    OBJECT_SCHEMA,
		    OBJECT_NAME,
		    COUNT_READ_NORMAL,
		    COUNT_READ_WITH_SHARED_LOCKS,
		    COUNT_READ_HIGH_PRIORITY,
		    COUNT_READ_NO_INSERT,
		    COUNT_READ_EXTERNAL,
		    COUNT_WRITE_ALLOW_WRITE,
		    COUNT_WRITE_CONCURRENT_INSERT,
		    COUNT_WRITE_LOW_PRIORITY,
		    COUNT_WRITE_NORMAL,
		    COUNT_WRITE_EXTERNAL,
		    SUM_TIMER_READ_NORMAL,
		    SUM_TIMER_READ_WITH_SHARED_LOCKS,
		    SUM_TIMER_READ_HIGH_PRIORITY,
		    SUM_TIMER_READ_NO_INSERT,
		    SUM_TIMER_READ_EXTERNAL,
		    SUM_TIMER_WRITE_ALLOW_WRITE,
		    SUM_TIMER_WRITE_CONCURRENT_INSERT,
		    SUM_TIMER_WRITE_LOW_PRIORITY,
		    SUM_TIMER_WRITE_NORMAL,
		    SUM_TIMER_WRITE_EXTERNAL
		  FROM performance_schema.table_lock_waits_summary_by_table
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema')
		`
	perfEventsStatementsQuery = `
		SELECT
		    ifnull(SCHEMA_NAME, 'NONE') as SCHEMA_NAME,
		    DIGEST,
		    LEFT(DIGEST_TEXT, %d) as DIGEST_TEXT,
		    COUNT_STAR,
		    SUM_TIMER_WAIT,
		    SUM_ERRORS,
		    SUM_WARNINGS,
		    SUM_ROWS_AFFECTED,
		    SUM_ROWS_SENT,
		    SUM_ROWS_EXAMINED,
		    SUM_CREATED_TMP_DISK_TABLES,
		    SUM_CREATED_TMP_TABLES,
		    SUM_SORT_MERGE_PASSES,
		    SUM_SORT_ROWS,
		    SUM_NO_INDEX_USED
		  FROM performance_schema.events_statements_summary_by_digest
		  WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema')
		    AND last_seen > DATE_SUB(NOW(), INTERVAL %d SECOND)
		  ORDER BY SUM_TIMER_WAIT DESC
		  LIMIT %d
		`
	perfEventsWaitsQuery = `
		SELECT EVENT_NAME, COUNT_STAR, SUM_TIMER_WAIT
		  FROM performance_schema.events_waits_summary_global_by_event_name
		`
	perfFileEventsQuery = `
		SELECT
		  EVENT_NAME,
		  COUNT_READ, SUM_TIMER_READ, SUM_NUMBER_OF_BYTES_READ,
		  COUNT_WRITE, SUM_TIMER_WRITE, SUM_NUMBER_OF_BYTES_WRITE,
		  COUNT_MISC, SUM_TIMER_MISC
		  FROM performance_schema.file_summary_by_event_name
		`
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

// Metric descriptors for dynamically created metrics.
var (
	performanceSchemaTableWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "table_io_waits_total"),
		"The total number of table I/O wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaTableWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "table_io_waits_seconds_total"),
		"The total time of table I/O wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaIndexWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "index_io_waits_total"),
		"The total number of index I/O wait events for each index and operation.",
		[]string{"schema", "name", "index", "operation"}, nil,
	)
	performanceSchemaIndexWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "index_io_waits_seconds_total"),
		"The total time of index I/O wait events for each index and operation.",
		[]string{"schema", "name", "index", "operation"}, nil,
	)
	performanceSchemaSQLTableLockWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "sql_lock_waits_total"),
		"The total number of SQL lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaExternalTableLockWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "external_lock_waits_total"),
		"The total number of external lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaSQLTableLockWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "sql_lock_waits_seconds_total"),
		"The total time of SQL lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaExternalTableLockWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "external_lock_waits_seconds_total"),
		"The total time of external lock wait events for each table and operation.",
		[]string{"schema", "name", "operation"}, nil,
	)
	performanceSchemaEventsStatementsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_total"),
		"The total count of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_seconds_total"),
		"The total time of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_errors_total"),
		"The errors of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsWarningsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_warnings_total"),
		"The warnings of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsAffectedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_affected_total"),
		"The total rows affected of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_sent_total"),
		"The total rows sent of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsRowsExaminedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_examined_total"),
		"The total rows examined of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTmpTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_tables_total"),
		"The total tmp tables of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTmpDiskTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_disk_tables_total"),
		"The total tmp disk tables of events statements by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsSortMergePassesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_sort_merge_passes_total"),
		"The total number of merge passes by the sort algorithm performed by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsSortRowsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_sort_rows_total"),
		"The total number of sorted rows by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsNoIndexUsedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_no_index_used_total"),
		"The total number of statements that used full table scans by digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_waits_total"),
		"The total events waits by event name.",
		[]string{"event_name"}, nil,
	)
	performanceSchemaEventsWaitsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_waits_seconds_total"),
		"The total seconds of events waits by event name.",
		[]string{"event_name"}, nil,
	)
	performanceSchemaFileEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_total"),
		"The total file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
	performanceSchemaFileEventsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_seconds_total"),
		"The total seconds of file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
	performanceSchemaFileEventsBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "file_events_bytes_total"),
		"The total bytes of file events by event name/mode.",
		[]string{"event_name", "mode"}, nil,
	)
)

// Math constants
const (
	picoSeconds = 1e12
)

// Various regexps.
var logRE = regexp.MustCompile(`.+\.(\d+)$`)

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
			Help:      "Total number of times an error occured scraping a MySQL.",
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
			log.Errorln("Error scraping collect.info_schema.tables:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tables").Inc()
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
		if err = scrapePerfTableIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tableiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tableiowaits").Inc()
		}
	}
	if *collectPerfIndexIOWaits {
		if err = scrapePerfIndexIOWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.indexiowaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.indexiowaits").Inc()
		}
	}
	if *collectPerfTableLockWaits {
		if err = scrapePerfTableLockWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.tablelocks:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.tablelocks").Inc()
		}
	}
	if *collectPerfEventsStatements {
		if err = scrapePerfEventsStatements(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventsstatements:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventsstatements").Inc()
		}
	}
	if *collectPerfEventsWaits {
		if err = scrapePerfEventsWaits(db, ch); err != nil {
			log.Errorln("Error scraping for collect.perf_schema.eventswaits:", err)
			e.scrapeErrors.WithLabelValues("collect.perf_schema.eventswaits").Inc()
		}
	}
	if *collectPerfFileEvents {
		if err = scrapePerfFileEvents(db, ch); err != nil {
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
	if *collectTableStat {
		if err = collector.ScrapeTableStat(db, ch); err != nil {
			log.Errorln("Error scraping table stat:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.tablestats").Inc()
		}
	}
	if *collectQueryResponseTime {
		if err = collector.ScrapeQueryResponseTime(db, ch); err != nil {
			log.Errorln("Error scraping query response time:", err)
			e.scrapeErrors.WithLabelValues("collect.info_schema.query_response_time").Inc()
		}
	}
	if *collectEngineTokudbStatus {
		if err = collector.ScrapeEngineTokudbStatus(db, ch); err != nil {
			log.Errorln("Error scraping TokuDB engine status:", err)
			e.scrapeErrors.WithLabelValues("collect.engine_tokudb_status").Inc()
		}
	}
}

func scrapePerfTableIOWaits(db *sql.DB, ch chan<- prometheus.Metric) error {
	perfSchemaTableWaitsRows, err := db.Query(perfTableIOWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableWaitsRows.Close()

	var (
		objectSchema, objectName                          string
		countFetch, countInsert, countUpdate, countDelete uint64
		timeFetch, timeInsert, timeUpdate, timeDelete     uint64
	)

	for perfSchemaTableWaitsRows.Next() {
		if err := perfSchemaTableWaitsRows.Scan(
			&objectSchema, &objectName, &countFetch, &countInsert, &countUpdate, &countDelete,
			&timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countFetch),
			objectSchema, objectName, "fetch",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countInsert),
			objectSchema, objectName, "insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countUpdate),
			objectSchema, objectName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countDelete),
			objectSchema, objectName, "delete",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeFetch)/picoSeconds,
			objectSchema, objectName, "fetch",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeInsert)/picoSeconds,
			objectSchema, objectName, "insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeUpdate)/picoSeconds,
			objectSchema, objectName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsTimeDesc, prometheus.CounterValue, float64(timeDelete)/picoSeconds,
			objectSchema, objectName, "delete",
		)
	}
	return nil
}

func scrapePerfIndexIOWaits(db *sql.DB, ch chan<- prometheus.Metric) error {
	perfSchemaIndexWaitsRows, err := db.Query(perfIndexIOWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaIndexWaitsRows.Close()

	var (
		objectSchema, objectName, indexName               string
		countFetch, countInsert, countUpdate, countDelete uint64
		timeFetch, timeInsert, timeUpdate, timeDelete     uint64
	)

	for perfSchemaIndexWaitsRows.Next() {
		if err := perfSchemaIndexWaitsRows.Scan(
			&objectSchema, &objectName, &indexName,
			&countFetch, &countInsert, &countUpdate, &countDelete,
			&timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countFetch),
			objectSchema, objectName, indexName, "fetch",
		)
		// We only include the insert column when indexName is NONE.
		if indexName == "NONE" {
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countInsert),
				objectSchema, objectName, indexName, "insert",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countUpdate),
			objectSchema, objectName, indexName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countDelete),
			objectSchema, objectName, indexName, "delete",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeFetch)/picoSeconds,
			objectSchema, objectName, indexName, "fetch",
		)
		// We only update write columns when indexName is NONE.
		if indexName == "NONE" {
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeInsert)/picoSeconds,
				objectSchema, objectName, indexName, "insert",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeUpdate)/picoSeconds,
			objectSchema, objectName, indexName, "update",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeDelete)/picoSeconds,
			objectSchema, objectName, indexName, "delete",
		)
	}
	return nil
}

func scrapePerfTableLockWaits(db *sql.DB, ch chan<- prometheus.Metric) error {
	perfSchemaTableLockWaitsRows, err := db.Query(perfTableLockWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableLockWaitsRows.Close()

	var (
		objectSchema               string
		objectName                 string
		countReadNormal            uint64
		countReadWithSharedLocks   uint64
		countReadHighPriority      uint64
		countReadNoInsert          uint64
		countReadExternal          uint64
		countWriteAllowWrite       uint64
		countWriteConcurrentInsert uint64
		countWriteLowPriority      uint64
		countWriteNormal           uint64
		countWriteExternal         uint64
		timeReadNormal             uint64
		timeReadWithSharedLocks    uint64
		timeReadHighPriority       uint64
		timeReadNoInsert           uint64
		timeReadExternal           uint64
		timeWriteAllowWrite        uint64
		timeWriteConcurrentInsert  uint64
		timeWriteLowPriority       uint64
		timeWriteNormal            uint64
		timeWriteExternal          uint64
	)

	for perfSchemaTableLockWaitsRows.Next() {
		if err := perfSchemaTableLockWaitsRows.Scan(
			&objectSchema,
			&objectName,
			&countReadNormal,
			&countReadWithSharedLocks,
			&countReadHighPriority,
			&countReadNoInsert,
			&countReadExternal,
			&countWriteAllowWrite,
			&countWriteConcurrentInsert,
			&countWriteLowPriority,
			&countWriteNormal,
			&countWriteExternal,
			&timeReadNormal,
			&timeReadWithSharedLocks,
			&timeReadHighPriority,
			&timeReadNoInsert,
			&timeReadExternal,
			&timeWriteAllowWrite,
			&timeWriteConcurrentInsert,
			&timeWriteLowPriority,
			&timeWriteNormal,
			&timeWriteExternal,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadNormal),
			objectSchema, objectName, "read_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadWithSharedLocks),
			objectSchema, objectName, "read_with_shared_locks",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadHighPriority),
			objectSchema, objectName, "read_high_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countReadNoInsert),
			objectSchema, objectName, "read_no_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteNormal),
			objectSchema, objectName, "write_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteAllowWrite),
			objectSchema, objectName, "write_allow_write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteConcurrentInsert),
			objectSchema, objectName, "write_concurrent_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteLowPriority),
			objectSchema, objectName, "write_low_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsDesc, prometheus.CounterValue, float64(countReadExternal),
			objectSchema, objectName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteExternal),
			objectSchema, objectName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadNormal)/picoSeconds,
			objectSchema, objectName, "read_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadWithSharedLocks)/picoSeconds,
			objectSchema, objectName, "read_with_shared_locks",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadHighPriority)/picoSeconds,
			objectSchema, objectName, "read_high_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadNoInsert)/picoSeconds,
			objectSchema, objectName, "read_no_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteNormal)/picoSeconds,
			objectSchema, objectName, "write_normal",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteAllowWrite)/picoSeconds,
			objectSchema, objectName, "write_allow_write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteConcurrentInsert)/picoSeconds,
			objectSchema, objectName, "write_concurrent_insert",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteLowPriority)/picoSeconds,
			objectSchema, objectName, "write_low_priority",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeReadExternal)/picoSeconds,
			objectSchema, objectName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaExternalTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteExternal)/picoSeconds,
			objectSchema, objectName, "write",
		)
	}
	return nil
}

func scrapePerfEventsStatements(db *sql.DB, ch chan<- prometheus.Metric) error {
	perfQuery := fmt.Sprintf(
		perfEventsStatementsQuery,
		*perfEventsStatementsDigestTextLimit,
		*perfEventsStatementsTimeLimit,
		*perfEventsStatementsLimit,
	)
	// Timers here are returned in picoseconds.
	perfSchemaEventsStatementsRows, err := db.Query(perfQuery)
	if err != nil {
		return err
	}
	defer perfSchemaEventsStatementsRows.Close()

	var (
		schemaName, digest, digestText       string
		count, queryTime, errors, warnings   uint64
		rowsAffected, rowsSent, rowsExamined uint64
		tmpTables, tmpDiskTables             uint64
		sortMergePasses, sortRows            uint64
		noIndexUsed                          uint64
	)

	for perfSchemaEventsStatementsRows.Next() {
		if err := perfSchemaEventsStatementsRows.Scan(
			&schemaName, &digest, &digestText, &count, &queryTime, &errors, &warnings, &rowsAffected, &rowsSent, &rowsExamined, &tmpTables, &tmpDiskTables, &sortMergePasses, &sortRows, &noIndexUsed,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsDesc, prometheus.CounterValue, float64(count),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTimeDesc, prometheus.CounterValue, float64(queryTime)/picoSeconds,
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsErrorsDesc, prometheus.CounterValue, float64(errors),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsWarningsDesc, prometheus.CounterValue, float64(warnings),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsAffectedDesc, prometheus.CounterValue, float64(rowsAffected),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsSentDesc, prometheus.CounterValue, float64(rowsSent),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsExaminedDesc, prometheus.CounterValue, float64(rowsExamined),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpTablesDesc, prometheus.CounterValue, float64(tmpTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpDiskTablesDesc, prometheus.CounterValue, float64(tmpDiskTables),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsSortMergePassesDesc, prometheus.CounterValue, float64(sortMergePasses),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsSortRowsDesc, prometheus.CounterValue, float64(sortRows),
			schemaName, digest, digestText,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsNoIndexUsedDesc, prometheus.CounterValue, float64(noIndexUsed),
			schemaName, digest, digestText,
		)
	}
	return nil
}

func scrapePerfEventsWaits(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaEventsWaitsRows, err := db.Query(perfEventsWaitsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaEventsWaitsRows.Close()

	var (
		eventName   string
		count, time uint64
	)

	for perfSchemaEventsWaitsRows.Next() {
		if err := perfSchemaEventsWaitsRows.Scan(
			&eventName, &count, &time,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsWaitsDesc, prometheus.CounterValue, float64(count),
			eventName,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsWaitsTimeDesc, prometheus.CounterValue, float64(time)/picoSeconds,
			eventName,
		)
	}
	return nil
}

func scrapePerfFileEvents(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaFileEventsRows, err := db.Query(perfFileEventsQuery)
	if err != nil {
		return err
	}
	defer perfSchemaFileEventsRows.Close()

	var (
		eventName                         string
		countRead, timeRead, bytesRead    uint64
		countWrite, timeWrite, bytesWrite uint64
		countMisc, timeMisc               uint64
	)

	for perfSchemaFileEventsRows.Next() {
		if err := perfSchemaFileEventsRows.Scan(
			&eventName,
			&countRead, &timeRead, &bytesRead,
			&countWrite, &timeWrite, &bytesWrite,
			&countMisc, &timeMisc,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countRead),
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeRead)/picoSeconds,
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsBytesDesc, prometheus.CounterValue, float64(bytesRead),
			eventName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countWrite),
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeWrite)/picoSeconds,
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsBytesDesc, prometheus.CounterValue, float64(bytesWrite),
			eventName, "write",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsDesc, prometheus.CounterValue, float64(countMisc),
			eventName, "misc",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaFileEventsTimeDesc, prometheus.CounterValue, float64(timeMisc)/picoSeconds,
			eventName, "misc",
		)
	}
	return nil
}

func newDesc(subsystem, name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, name),
		help, nil, nil,
	)
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

func main() {
	flag.Parse()

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

	log.Infof("Starting Server: %s", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
