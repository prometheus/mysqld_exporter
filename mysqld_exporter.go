package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
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
	autoIncrementColumns = flag.Bool(
		"collect.auto_increment.columns", false,
		"Collect auto_increment columns and max values from information_schema",
	)
	binlogSize = flag.Bool(
		"collect.binlog_size", false,
		"Collect the current size of all registered binlog files",
	)
	perfTableIOWaits = flag.Bool(
		"collect.perf_schema.tableiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_table",
	)
	perfTableIOWaitsTime = flag.Bool(
		"collect.perf_schema.tableiowaitstime", false,
		"Collect time metrics from performance_schema.table_io_waits_summary_by_table",
	)
	perfIndexIOWaits = flag.Bool(
		"collect.perf_schema.indexiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_index_usage",
	)
	perfIndexIOWaitsTime = flag.Bool(
		"collect.perf_schema.indexiowaitstime", false,
		"Collect time metrics from performance_schema.table_io_waits_summary_by_index_usage",
	)
	perfTableLockWaits = flag.Bool(
		"collect.perf_schema.tablelocks", false,
		"Collect metrics from performance_schema.table_lock_waits_summary_by_table",
	)
	perfTableLockWaitsTime = flag.Bool(
		"collect.perf_schema.tablelockstime", false,
		"Collect time metrics from performance_schema.table_lock_waits_summary_by_table",
	)
	perfEventsStatements = flag.Bool(
		"collect.perf_schema.eventsstatements", false,
		"Collect time metrics from performance_schema.events_statements_summary_by_digest",
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
	userStat = flag.Bool("collect.info_schema.userstats", false,
		"If running with userstat=1, set to true to collect user statistics",
	)
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "mysql"
	// Subsystems.
	exporter          = "exporter"
	globalStatus      = "global_status"
	globalVariables   = "global_variables"
	informationSchema = "info_schema"
	performanceSchema = "perf_schema"
	slaveStatus       = "slave_status"
	binlog            = "binlog"
)

// Metric SQL Queries.
const (
	globalStatusQuery            = `SHOW GLOBAL STATUS`
	globalVariablesQuery         = `SHOW GLOBAL VARIABLES`
	slaveStatusQuery             = `SHOW SLAVE STATUS`
	binlogQuery                  = `SHOW BINARY LOGS`
	infoSchemaAutoIncrementQuery = `
		SELECT table_schema, table_name, column_name, auto_increment,
		  pow(2, case data_type
		    when 'tinyint'   then 7
		    when 'smallint'  then 15
		    when 'mediumint' then 23
		    when 'int'       then 31
		    when 'bigint'    then 63
		    end+(column_type like '% unsigned'))-1 as max_int
		  FROM information_schema.tables t
		  JOIN information_schema.columns c USING (table_schema,table_name)
		  WHERE c.extra = 'auto_increment' AND t.auto_increment IS NOT NULL
		`
	perfTableIOWaitsQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE
		  FROM performance_schema.table_io_waits_summary_by_table
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
		`
	perfTableIOWaitsTimeQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
		  FROM performance_schema.table_io_waits_summary_by_table
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
		`
	perfIndexIOWaitsQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE
		  FROM performance_schema.table_io_waits_summary_by_index_usage
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')
		`
	perfIndexIOWaitsTimeQuery = `
		SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME, SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE
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
		    COUNT_WRITE_NORMAL,
		    COUNT_WRITE_ALLOW_WRITE,
		    COUNT_WRITE_CONCURRENT_INSERT,
		    COUNT_WRITE_DELAYED,
		    COUNT_WRITE_LOW_PRIORITY,
		    COUNT_READ_EXTERNAL,
		    COUNT_WRITE_EXTERNAL
		  FROM performance_schema.table_lock_waits_summary_by_table
		  WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema')
		`
	perfTableLockWaitsTimeQuery = `
		SELECT
		    OBJECT_SCHEMA,
		    OBJECT_NAME,
		    SUM_TIMER_READ_NORMAL,
		    SUM_TIMER_READ_WITH_SHARED_LOCKS,
		    SUM_TIMER_READ_HIGH_PRIORITY,
		    SUM_TIMER_READ_NO_INSERT,
		    SUM_TIMER_WRITE_NORMAL,
		    SUM_TIMER_WRITE_ALLOW_WRITE,
		    SUM_TIMER_WRITE_CONCURRENT_INSERT,
		    SUM_TIMER_WRITE_DELAYED,
		    SUM_TIMER_WRITE_LOW_PRIORITY,
		    SUM_TIMER_READ_EXTERNAL,
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
		    SUM_CREATED_TMP_TABLES
		  FROM performance_schema.events_statements_summary_by_digest
		  WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema')
		    AND last_seen > DATE_SUB(NOW(), INTERVAL %d SECOND)
		  ORDER BY SUM_TIMER_WAIT DESC
		  LIMIT %d
		`
	userStatQuery = `SELECT * FROM information_schema.USER_STATISTICS`
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
	binlogSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, binlog, "size"),
		"Combined size of all registered binlog files.",
		[]string{}, nil,
	)
	binlogFilesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, binlog, "files"),
		"Number of registered binlog files.",
		[]string{}, nil,
	)
	globalCommandsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "commands_total"),
		"Total number of executed MySQL commands.",
		[]string{"command"}, nil,
	)
	globalConnectionErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "connection_errors_total"),
		"Total number of MySQL connection errors.",
		[]string{"error"}, nil,
	)
	globalInnoDBRowOpsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "innodb_row_ops_total"),
		"Total number of MySQL InnoDB row operations.",
		[]string{"operation"}, nil,
	)
	globalInfoSchemaAutoIncrementDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "auto_increment_column"),
		"The current value of an auto_increment column from information_schema.",
		[]string{"schema", "table", "column"}, nil,
	)
	globalInfoSchemaAutoIncrementMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "auto_increment_column_max"),
		"The max value of an auto_increment column from information_schema.",
		[]string{"schema", "table", "column"}, nil,
	)
	globalPerformanceSchemaLostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "performance_schema_lost_total"),
		"Total number of MySQL instrumentations that could not be loaded or created due to memory constraints.",
		[]string{"instrumentation"}, nil,
	)
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
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsDigestTextDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_digest_text"),
		"The mapping of query schema text to their digest.",
		[]string{"schema", "digest", "digest_text"}, nil,
	)
	performanceSchemaEventsStatementsTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_seconds_total"),
		"The total time of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsErrorsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_errors_total"),
		"The errors of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsWarningsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_warnings_total"),
		"The warnings of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsRowsAffectedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_affected_total"),
		"The total rows affected of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsRowsSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_sent_total"),
		"The total rows sent of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsRowsExaminedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_rows_examined_total"),
		"The total rows examined of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsTmpTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_tables_total"),
		"The total tmp tables of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	performanceSchemaEventsStatementsTmpDiskTablesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "events_statements_tmp_disk_tables_total"),
		"The total tmp disk tables of events statements by digest.",
		[]string{"schema", "digest"}, nil,
	)
	// Map known user-statistics values to types. Unknown types will be mapped as
	// untyped.
	informationSchemaUserStatisticsTypes = map[string]struct {
		vtype prometheus.ValueType
		desc  *prometheus.Desc
	}{
		"TOTAL_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_total_connections"),
				"The number of connections created for this user.",
				[]string{"user"}, nil)},
		"CONCURRENT_CONNECTIONS": {prometheus.GaugeValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_concurrent_connections"),
				"The number of concurrent connections for this user.",
				[]string{"user"}, nil)},
		"CONNECTED_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_connected_time"),
				"The cumulative number of seconds elapsed while there were connections from this user.",
				[]string{"user"}, nil)},
		"BUSY_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_busy_time"),
				"The cumulative number of seconds there was activity on connections from this user.",
				[]string{"user"}, nil)},
		"CPU_TIME": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_cpu_time"),
				"The cumulative CPU time elapsed, in seconds, while servicing this user's connections.",
				[]string{"user"}, nil)},
		"BYTES_RECEIVED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_bytes_received"),
				"The number of bytes received from this user’s connections.",
				[]string{"user"}, nil)},
		"BYTES_SENT": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_bytes_sent"),
				"The number of bytes sent to this user’s connections.",
				[]string{"user"}, nil)},
		"BINLOG_BYTES_WRITTEN": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_binlog_bytes_written"),
				"The number of bytes written to the binary log from this user’s connections.",
				[]string{"user"}, nil)},
		"ROWS_FETCHED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_rows_fetched"),
				"The number of rows fetched by this user’s connections.",
				[]string{"user"}, nil)},
		"ROWS_UPDATED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_rows_updated"),
				"The number of rows updated by this user’s connections.",
				[]string{"user"}, nil)},
		"TABLE_ROWS_READ": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_table_rows_read"),
				"The number of rows read from tables by this user’s connections. (It may be different from ROWS_FETCHED.)",
				[]string{"user"}, nil)},
		"SELECT_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_select_commands"),
				"The number of SELECT commands executed from this user’s connections.",
				[]string{"user"}, nil)},
		"UPDATE_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_update_commands"),
				"The number of UPDATE commands executed from this user’s connections.",
				[]string{"user"}, nil)},
		"OTHER_COMMANDS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_other_commands"),
				"The number of other commands executed from this user’s connections.",
				[]string{"user"}, nil)},
		"COMMIT_TRANSACTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_commit_transactions"),
				"The number of COMMIT commands issued by this user’s connections.",
				[]string{"user"}, nil)},
		"ROLLBACK_TRANSACTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_rollback_transactions"),
				"The number of ROLLBACK commands issued by this user’s connections.",
				[]string{"user"}, nil)},
		"DENIED_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_denied_connections"),
				"The number of connections denied to this user.",
				[]string{"user"}, nil)},
		"LOST_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_lost_connections"),
				"The number of this user’s connections that were terminated uncleanly.",
				[]string{"user"}, nil)},
		"ACCESS_DENIED": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_access_denied"),
				"The number of times this user’s connections issued commands that were denied.",
				[]string{"user"}, nil)},
		"EMPTY_QUERIES": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_empty_queries"),
				"The number of times this user’s connections sent empty queries to the server.",
				[]string{"user"}, nil)},
		"TOTAL_SSL_CONNECTIONS": {prometheus.CounterValue,
			prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, "user_statistics_total_ssl_connections"),
				"The number of times this user’s connections connected using SSL to the server.",
				[]string{"user"}, nil)},
	}
)

// Math constants
const (
	picoSeconds = 1e12
)

// Various regexps.
var (
	globalStatusRE = regexp.MustCompile(`^(com|connection_errors|innodb_rows|performance_schema)_(.*)$`)
	logRE          = regexp.MustCompile(`.+\.(\d+)$`)
)

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	dsn             string
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
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

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
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
		log.Println("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	if err = scrapeGlobalStatus(db, ch); err != nil {
		log.Println("Error scraping global state:", err)
		return
	}
	if err = scrapeGlobalVariables(db, ch); err != nil {
		log.Println("Error scraping global variables:", err)
		return
	}
	if err = scrapeSlaveStatus(db, ch); err != nil {
		log.Println("Error scraping slave state:", err)
		return
	}
	if *autoIncrementColumns {
		if err = scrapeInformationSchema(db, ch); err != nil {
			log.Println("Error scraping information schema:", err)
			return
		}
	}
	if *binlogSize {
		if err = scrapeBinlogSize(db, ch); err != nil {
			log.Println("Error scraping binlog size:", err)
			return
		}
	}
	if *perfTableIOWaits {
		if err = scrapePerfTableIOWaits(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfTableIOWaitsTime {
		if err = scrapePerfTableIOWaitsTime(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfIndexIOWaits {
		if err = scrapePerfIndexIOWaits(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfIndexIOWaitsTime {
		if err = scrapePerfIndexIOWaitsTime(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfTableLockWaits {
		if err = scrapePerfTableLockWaits(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfTableLockWaitsTime {
		if err = scrapePerfTableLockWaitsTime(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *perfEventsStatements {
		if err = scrapePerfEventsStatements(db, ch); err != nil {
			log.Println("Error scraping performance schema:", err)
			return
		}
	}
	if *userStat {
		if err = scrapeUserStat(db, ch); err != nil {
			log.Println("Error scraping user stat:", err)
			return
		}
	}
}

func scrapeGlobalStatus(db *sql.DB, ch chan<- prometheus.Metric) error {
	globalStatusRows, err := db.Query(globalStatusQuery)
	if err != nil {
		return err
	}
	defer globalStatusRows.Close()

	var key string
	var val sql.RawBytes

	for globalStatusRows.Next() {
		if err := globalStatusRows.Scan(&key, &val); err != nil {
			return err
		}
		if floatVal, ok := parseStatus(val); ok { // Unparsable values are silently skipped.
			key = strings.ToLower(key)
			match := globalStatusRE.FindStringSubmatch(key)
			if match == nil {
				ch <- prometheus.MustNewConstMetric(
					newDesc(globalStatus, key, "Generic metric from SHOW GLOBAL STATUS."),
					prometheus.UntypedValue,
					floatVal,
				)
				continue
			}
			switch match[1] {
			case "com":
				ch <- prometheus.MustNewConstMetric(
					globalCommandsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "connection_errors":
				ch <- prometheus.MustNewConstMetric(
					globalConnectionErrorsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "innodb_rows":
				ch <- prometheus.MustNewConstMetric(
					globalInnoDBRowOpsDesc, prometheus.CounterValue, floatVal, match[2],
				)
			case "performance_schema":
				ch <- prometheus.MustNewConstMetric(
					globalPerformanceSchemaLostDesc, prometheus.CounterValue, floatVal, match[2],
				)
			}
		}
	}
	return nil
}

func scrapeGlobalVariables(db *sql.DB, ch chan<- prometheus.Metric) error {
	globalVariablesRows, err := db.Query(globalVariablesQuery)
	if err != nil {
		return err
	}
	defer globalVariablesRows.Close()

	var key string
	var val sql.RawBytes

	for globalVariablesRows.Next() {
		if err := globalVariablesRows.Scan(&key, &val); err != nil {
			return err
		}
		key = strings.ToLower(key)
		if floatVal, ok := parseStatus(val); ok {
			ch <- prometheus.MustNewConstMetric(
				newDesc(globalVariables, key, "Generic gauge metric from SHOW GLOBAL VARIABLES."),
				prometheus.GaugeValue,
				floatVal,
			)
			continue
		}
	}
	return nil
}

func scrapeSlaveStatus(db *sql.DB, ch chan<- prometheus.Metric) error {
	slaveStatusRows, err := db.Query(slaveStatusQuery)
	if err != nil {
		return err
	}
	defer slaveStatusRows.Close()

	if slaveStatusRows.Next() {
		// There is either no row in SHOW SLAVE STATUS (if this is not a
		// slave server), or exactly one. In case of multi-source
		// replication, things work very much differently. This code
		// cannot deal with that case.
		slaveCols, err := slaveStatusRows.Columns()
		if err != nil {
			return err
		}

		// As the number of columns varies with mysqld versions,
		// and sql.Scan requires []interface{}, we need to create a
		// slice of pointers to the elements of slaveData.
		scanArgs := make([]interface{}, len(slaveCols))
		for i := range scanArgs {
			scanArgs[i] = &sql.RawBytes{}
		}

		if err := slaveStatusRows.Scan(scanArgs...); err != nil {
			return err
		}
		for i, col := range slaveCols {
			if value, ok := parseStatus(*scanArgs[i].(*sql.RawBytes)); ok { // Silently skip unparsable values.
				ch <- prometheus.MustNewConstMetric(
					newDesc(slaveStatus, strings.ToLower(col), "Generic metric from SHOW SLAVE STATUS."),
					prometheus.UntypedValue,
					value,
				)
			}
		}
	}
	return nil
}

func scrapeInformationSchema(db *sql.DB, ch chan<- prometheus.Metric) error {
	autoIncrementRows, err := db.Query(infoSchemaAutoIncrementQuery)
	if err != nil {
		return err
	}
	defer autoIncrementRows.Close()

	var (
		schema, table, column string
		value, max            uint64
	)

	for autoIncrementRows.Next() {
		if err := autoIncrementRows.Scan(
			&schema, &table, &column, &value, &max,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			globalInfoSchemaAutoIncrementDesc, prometheus.GaugeValue, float64(value),
			schema, table, column,
		)
		ch <- prometheus.MustNewConstMetric(
			globalInfoSchemaAutoIncrementMaxDesc, prometheus.GaugeValue, float64(max),
			schema, table, column,
		)
	}
	return nil
}

func scrapeBinlogSize(db *sql.DB, ch chan<- prometheus.Metric) error {
	masterLogRows, err := db.Query(binlogQuery)
	if err != nil {
		return err
	}
	defer masterLogRows.Close()

	var (
		size     uint64
		count    uint64
		filename string
		filesize uint64
	)
	size = 0
	count = 0

	for masterLogRows.Next() {
		if err := masterLogRows.Scan(&filename, &filesize); err != nil {
			return nil
		}
		size += filesize
		count++
	}

	ch <- prometheus.MustNewConstMetric(
		binlogSizeDesc, prometheus.GaugeValue, float64(size),
	)
	ch <- prometheus.MustNewConstMetric(
		binlogFilesDesc, prometheus.GaugeValue, float64(count),
	)

	return nil
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
	)

	for perfSchemaTableWaitsRows.Next() {
		if err := perfSchemaTableWaitsRows.Scan(
			&objectSchema, &objectName, &countFetch, &countInsert, &countUpdate, &countDelete,
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
	}
	return nil
}

func scrapePerfTableIOWaitsTime(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaTableWaitsTimeRows, err := db.Query(perfTableIOWaitsTimeQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableWaitsTimeRows.Close()

	var (
		objectSchema, objectName                      string
		timeFetch, timeInsert, timeUpdate, timeDelete uint64
	)

	for perfSchemaTableWaitsTimeRows.Next() {
		if err := perfSchemaTableWaitsTimeRows.Scan(
			&objectSchema, &objectName, &timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
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
	)

	for perfSchemaIndexWaitsRows.Next() {
		if err := perfSchemaIndexWaitsRows.Scan(
			&objectSchema, &objectName, &indexName, &countFetch, &countInsert, &countUpdate, &countDelete,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countFetch),
			objectSchema, objectName, indexName, "fetch",
		)
		// We only update write columns when indexName is NONE.
		if indexName == "NONE" {
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countInsert),
				objectSchema, objectName, indexName, "insert",
			)
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countUpdate),
				objectSchema, objectName, indexName, "update",
			)
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsDesc, prometheus.CounterValue, float64(countDelete),
				objectSchema, objectName, indexName, "delete",
			)
		}
	}
	return nil
}

func scrapePerfIndexIOWaitsTime(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaIndexWaitsTimeRows, err := db.Query(perfIndexIOWaitsTimeQuery)
	if err != nil {
		return err
	}
	defer perfSchemaIndexWaitsTimeRows.Close()

	var (
		objectSchema, objectName, indexName           string
		timeFetch, timeInsert, timeUpdate, timeDelete uint64
	)

	for perfSchemaIndexWaitsTimeRows.Next() {
		if err := perfSchemaIndexWaitsTimeRows.Scan(
			&objectSchema, &objectName, &indexName, &timeFetch, &timeInsert, &timeUpdate, &timeDelete,
		); err != nil {
			return err
		}
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
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeUpdate)/picoSeconds,
				objectSchema, objectName, indexName, "update",
			)
			ch <- prometheus.MustNewConstMetric(
				performanceSchemaIndexWaitsTimeDesc, prometheus.CounterValue, float64(timeDelete)/picoSeconds,
				objectSchema, objectName, indexName, "delete",
			)
		}
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
		objectSchema, objectName                      string
		countReadNormal, countReadWithSharedLocks     uint64
		countReadHighPriority, countReadNoInsert      uint64
		countWriteNormal, countWriteAllowWrite        uint64
		countWriteConcurrentInsert, countWriteDelayed uint64
		countWriteLowPriority                         uint64
		countReadExternal, countWriteExternal         uint64
	)

	for perfSchemaTableLockWaitsRows.Next() {
		if err := perfSchemaTableLockWaitsRows.Scan(
			&objectSchema, &objectName, &countReadNormal, &countReadWithSharedLocks,
			&countReadHighPriority, &countReadNoInsert, &countWriteNormal,
			&countWriteAllowWrite, &countWriteConcurrentInsert, &countWriteDelayed,
			&countWriteLowPriority, &countReadExternal, &countWriteExternal,
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
			performanceSchemaSQLTableLockWaitsDesc, prometheus.CounterValue, float64(countWriteDelayed),
			objectSchema, objectName, "write_delayed",
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
	}
	return nil
}

func scrapePerfTableLockWaitsTime(db *sql.DB, ch chan<- prometheus.Metric) error {
	// Timers here are returned in picoseconds.
	perfSchemaTableWaitsTimeRows, err := db.Query(perfTableLockWaitsTimeQuery)
	if err != nil {
		return err
	}
	defer perfSchemaTableWaitsTimeRows.Close()

	var (
		objectSchema, objectName                    string
		timeReadNormal, timeReadWithSharedLocks     uint64
		timeReadHighPriority, timeReadNoInsert      uint64
		timeWriteNormal, timeWriteAllowWrite        uint64
		timeWriteConcurrentInsert, timeWriteDelayed uint64
		timeWriteLowPriority                        uint64
		timeReadExternal, timeWriteExternal         uint64
	)

	for perfSchemaTableWaitsTimeRows.Next() {
		if err := perfSchemaTableWaitsTimeRows.Scan(
			&objectSchema, &objectName, &timeReadNormal, &timeReadWithSharedLocks,
			&timeReadHighPriority, &timeReadNoInsert, &timeWriteNormal,
			&timeWriteAllowWrite, &timeWriteConcurrentInsert, &timeWriteDelayed,
			&timeWriteLowPriority, &timeReadExternal, &timeWriteExternal,
		); err != nil {
			return err
		}
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
			performanceSchemaSQLTableLockWaitsTimeDesc, prometheus.CounterValue, float64(timeWriteDelayed)/picoSeconds,
			objectSchema, objectName, "write_delayed",
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
		schemaName, digest, digest_text      string
		count, queryTime, errors, warnings   uint64
		rowsAffected, rowsSent, rowsExamined uint64
		tmpTables, tmpDiskTables             uint64
	)

	for perfSchemaEventsStatementsRows.Next() {
		if err := perfSchemaEventsStatementsRows.Scan(
			&schemaName, &digest, &digest_text, &count, &queryTime, &errors, &warnings, &rowsAffected, &rowsSent, &rowsExamined, &tmpTables, &tmpDiskTables,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsDesc, prometheus.CounterValue, float64(count),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsDigestTextDesc, prometheus.GaugeValue, 1,
			schemaName, digest, digest_text,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTimeDesc, prometheus.CounterValue, float64(queryTime)/picoSeconds,
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsErrorsDesc, prometheus.CounterValue, float64(errors),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsWarningsDesc, prometheus.CounterValue, float64(warnings),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsAffectedDesc, prometheus.CounterValue, float64(rowsAffected),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsSentDesc, prometheus.CounterValue, float64(rowsSent),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsRowsExaminedDesc, prometheus.CounterValue, float64(rowsExamined),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpTablesDesc, prometheus.CounterValue, float64(tmpTables),
			schemaName, digest,
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaEventsStatementsTmpDiskTablesDesc, prometheus.CounterValue, float64(tmpDiskTables),
			schemaName, digest,
		)
	}
	return nil
}

func scrapeUserStat(db *sql.DB, ch chan<- prometheus.Metric) error {
	informationSchemaUserStatisticsRows, err := db.Query(userStatQuery)
	if err != nil {
		return err
	}
	defer informationSchemaUserStatisticsRows.Close()

	// The user column is assumed to be column[0], while all other data is assumed to be coerceable to float64.
	// Because of the user column, userStatData[0] maps to columnNames[1] when reading off the metrics
	// (because userStatScanArgs is mapped as [ &user, &userData[0], &userData[1] ... &userdata[n] ]
	// To map metrics to names therefore we always range over columnNames[1:]
	var columnNames []string
	columnNames, err = informationSchemaUserStatisticsRows.Columns()
	if err != nil {
		return err
	}

	var user string                                        // Holds the username, which should be in column 0.
	var userStatData = make([]float64, len(columnNames)-1) // 1 less because of the user column.
	var userStatScanArgs = make([]interface{}, len(columnNames))
	userStatScanArgs[0] = &user
	for i := range userStatData {
		userStatScanArgs[i+1] = &userStatData[i]
	}

	for informationSchemaUserStatisticsRows.Next() {
		err = informationSchemaUserStatisticsRows.Scan(userStatScanArgs...)
		if err != nil {
			return err
		}

		// Loop over column names, and match to scan data. Unknown columns
		// will be filled with an untyped metric number. We assume other then
		// user, that we'll only get numbers.
		for idx, columnName := range columnNames[1:] {
			if metricType, ok := informationSchemaUserStatisticsTypes[columnName]; ok {
				ch <- prometheus.MustNewConstMetric(metricType.desc, metricType.vtype, float64(userStatData[idx]), user)
			} else {
				// Unknown metric. Report as untyped.
				desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, informationSchema, fmt.Sprintf("user_statistics_%s", strings.ToLower(columnName))), fmt.Sprintf("Unsupported metric from column %s", columnName), []string{"user"}, nil)
				ch <- prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, float64(userStatData[idx]), user)
			}
		}
	}
	return nil
}

func newDesc(subsystem, name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, name),
		help, nil, nil,
	)
}

func parseStatus(data sql.RawBytes) (float64, bool) {
	if bytes.Compare(data, []byte("Yes")) == 0 || bytes.Compare(data, []byte("ON")) == 0 {
		return 1, true
	}
	if bytes.Compare(data, []byte("No")) == 0 || bytes.Compare(data, []byte("OFF")) == 0 {
		return 0, true
	}
	if logNum := logRE.Find(data); logNum != nil {
		value, err := strconv.ParseFloat(string(logNum), 64)
		return value, err == nil
	}
	value, err := strconv.ParseFloat(string(data), 64)
	return value, err == nil
}

func main() {
	flag.Parse()

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if len(dsn) == 0 {
		log.Fatal("couldn't find environment variable DATA_SOURCE_NAME")
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
