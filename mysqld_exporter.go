package main

import (
	"bytes"
	"database/sql"
	"flag"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "mysql"
	// Subsystems.
	exporter          = "exporter"
	slaveStatus       = "slave_status"
	globalStatus      = "global_status"
	performanceSchema = "perf_schema"
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
	globalPerformanceSchemaLostDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, globalStatus, "performance_schema_lost_total"),
		"Total number of MySQL instrumentations that could not be loaded or created due to memory constraints.",
		[]string{"instrumentation"}, nil,
	)
	performanceSchemaTableWaitsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, performanceSchema, "table_io_waits_total"),
		"The total number of table I/O wait events for each table and operation.",
		[]string{"schema", "name", "operations"}, nil,
	)
)

// Various regexps.
var (
	globalStatusRE = regexp.MustCompile(`^(com|connection_errors|innodb_rows|performance_schema)_(.*)$`)
	logRE          = regexp.MustCompile(`.+\.(\d+)$`)
)

// Exporter collects MySQL metrics. It implements prometheus.Collector.
type Exporter struct {
	mutex           sync.Mutex
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
	// here is that we need to connecte to the MySQL DB. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored MySQL instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
	}()

	e.Collect(metricCh)
	close(metricCh)
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
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

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
	}(time.Now())

	e.error.Set(0)
	e.totalScrapes.Inc()

	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Println("Error opening connection to database:", err)
		e.error.Set(1)
		return
	}
	defer db.Close()

	globalStatusRows, err := db.Query("SHOW GLOBAL STATUS")
	if err != nil {
		log.Println("Error running status query on database:", err)
		e.error.Set(1)
		return
	}
	defer globalStatusRows.Close()

	var key string
	var val sql.RawBytes

	for globalStatusRows.Next() {
		if err := globalStatusRows.Scan(&key, &val); err != nil {
			log.Println("Error getting result set:", err)
			e.error.Set(1)
			return
		}
		if floatVal, ok := parseStatus(val); ok {
			match := globalStatusRE.FindStringSubmatch(key)
			if match == nil {
				ch <- prometheus.MustNewConstMetric(
					newDesc(globalStatus, strings.ToLower(key), "Generic metric from SHOW GLOBAL STATUS."),
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

	slaveStatusRows, err := db.Query("SHOW SLAVE STATUS")
	if err != nil {
		log.Println("Error running show slave query on database:", err)
		e.error.Set(1)
		return
	}
	defer slaveStatusRows.Close()

	if slaveStatusRows.Next() {
		// There is either no row in SHOW SLAVE STATUS (if this is not a
		// slave server), or exactly one. In case of multi-source
		// replication, things work very much differently. This code
		// cannot deal with that case.
		slaveCols, err := slaveStatusRows.Columns()
		if err != nil {
			log.Println("Error retrieving column list:", err)
			e.error.Set(1)
			return
		}

		// As the number of columns varies with mysqld versions,
		// and sql.Scan requires []interface{}, we need to create a
		// slice of pointers to the elements of slaveData.
		scanArgs := make([]interface{}, len(slaveCols))
		for i := range scanArgs {
			scanArgs[i] = &sql.RawBytes{}
		}

		if err := slaveStatusRows.Scan(scanArgs...); err != nil {
			log.Println("Error retrieving result set:", err)
			e.error.Set(1)
			return
		}
		for i, col := range slaveCols {
			if value, ok := parseStatus(*scanArgs[i].(*sql.RawBytes)); ok {
				ch <- prometheus.MustNewConstMetric(
					newDesc(slaveStatus, strings.ToLower(col), "Generic metric from SHOW SLAVE STATUS."),
					prometheus.UntypedValue,
					value,
				)
			}
		}
	}

	perfSchemaTableWaitsRows, err := db.Query("SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_READ, COUNT_WRITE, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE FROM performance_schema.table_io_waits_summary_by_table WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')")
	if err != nil {
		log.Println("Error running performance schema query on database:", err)
		e.error.Set(1)
		return
	}
	defer perfSchemaTableWaitsRows.Close()

	var (
		objectSchema string
		objectName   string
		countRead    int64
		countWrite   int64
		countFetch   int64
		countInsert  int64
		countUpdate  int64
		countDelete  int64
	)

	for perfSchemaTableWaitsRows.Next() {
		if err := perfSchemaTableWaitsRows.Scan(
			&objectSchema, &objectName, &countRead, &countWrite, &countFetch, &countInsert, &countUpdate, &countDelete,
		); err != nil {
			log.Println("error getting result set:", err)
			return
		}
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countRead),
			objectSchema, objectName, "read",
		)
		ch <- prometheus.MustNewConstMetric(
			performanceSchemaTableWaitsDesc, prometheus.CounterValue, float64(countWrite),
			objectSchema, objectName, "write",
		)
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
