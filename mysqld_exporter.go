package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
)

const (
	namespace         = "mysql"
	slaveStatus       = "slave_status"
	globalStatus      = "global_status"
	performanceSchema = "perf_schema"
)

var (
	listenAddress = flag.String("web.listen-address", ":9104", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

type Exporter struct {
	dsn                         string
	mutex                       sync.RWMutex
	duration, error             prometheus.Gauge
	totalScrapes                prometheus.Counter
	metrics                     map[string]prometheus.Gauge
	commands                    *prometheus.CounterVec
	connectionErrors            *prometheus.CounterVec
	innodbRows                  *prometheus.CounterVec
	globalPerformanceSchema     *prometheus.CounterVec
	performanceSchemaTableWaits *prometheus.CounterVec
}

// return new empty exporter
func NewMySQLExporter(dsn string) *Exporter {
	return &Exporter{
		dsn: dsn,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "exporter_last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total mysqld scrapes.",
		}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "exporter_last_scrape_error",
			Help:      "The last scrape error status.",
		}),
		metrics: map[string]prometheus.Gauge{},
		commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: globalStatus,
			Name:      "commands_total",
			Help:      "Number of executed mysql commands.",
		}, []string{"command"}),
		connectionErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: globalStatus,
			Name:      "connection_errors_total",
			Help:      "Number of mysql connection errors.",
		}, []string{"error"}),
		innodbRows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: globalStatus,
			Name:      "innodb_rows_total",
			Help:      "Mysql Innodb row operations.",
		}, []string{"operation"}),
		globalPerformanceSchema: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: globalStatus,
			Name:      "performance_schema_total",
			Help:      "Mysql instrumentations that could not be loaded or created due to memory constraints.",
		}, []string{"instrumentation"}),
		performanceSchemaTableWaits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: performanceSchema,
			Name:      "table_io_waits",
			Help:      "Mysql performance_schema.table_io_waits_summary_by_table",
		}, []string{"performance_schema"}),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.metrics {
		m.Describe(ch)
	}

	e.commands.Describe(ch)
	e.connectionErrors.Describe(ch)
	e.innodbRows.Describe(ch)
	e.globalPerformanceSchema.Describe(ch)
	e.performanceSchemaTableWaits.Describe(ch)

	ch <- e.duration.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.error.Desc()
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	scrapes := make(chan metric)

	go e.scrape(scrapes)

	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.setMetrics(scrapes)
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	e.collectMetrics(ch)
	e.commands.Collect(ch)
	e.connectionErrors.Collect(ch)
	e.innodbRows.Collect(ch)
	e.globalPerformanceSchema.Collect(ch)
	e.performanceSchemaTableWaits.Collect(ch)
}

type metric struct {
	source string
	key    string
	value  string
}

func (e *Exporter) scrape(scrapes chan<- metric) {
	defer close(scrapes)

	now := time.Now().UnixNano()

	e.totalScrapes.Inc()

	// Setup the database connection
	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Println("error opening connection to database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer db.Close()

	// fetch server global status
	globalStatusRows, err := db.Query("SHOW GLOBAL STATUS")
	if err != nil {
		log.Println("error running status query on database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer globalStatusRows.Close()

	var key, val []byte

	for globalStatusRows.Next() {
		// get RawBytes from data
		err = globalStatusRows.Scan(&key, &val)
		if err != nil {
			log.Println("error getting result set:", err)
			return
		}

		res := metric{
			source: globalStatus,
			key:    string(key),
			value:  string(val),
		}

		scrapes <- res
	}

	// fetch slave status
	slaveStatusRows, err := db.Query("SHOW SLAVE STATUS")
	if err != nil {
		log.Println("error running show slave query on database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer slaveStatusRows.Close()

	var slaveCols []string

	slaveCols, err = slaveStatusRows.Columns()
	if err != nil {
		log.Println("error retrieving column list:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}

	var slaveData = make([]sql.RawBytes, len(slaveCols))

	// As the number of columns varies with mysqld versions,
	// and sql.Scan requires []interface{}, we need to create a
	// slice of pointers to the elements of slaveData.

	scanArgs := make([]interface{}, len(slaveCols))
	for i := range slaveData {
		scanArgs[i] = &slaveData[i]
	}

	for slaveStatusRows.Next() {

		err = slaveStatusRows.Scan(scanArgs...)
		if err != nil {
			log.Println("error retrieving result set:", err)
			e.error.Set(1)
			e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
			return
		}

	}

	for i, col := range slaveCols {

		res := metric{
			source: slaveStatus,
			key:    col,
			value:  parseStatus(slaveData[i]),
		}

		scrapes <- res

	}

	// fetch performance_schema.table_io_waits_summary_by_table
	perfSchemaTableWaitsRows, err := db.Query("SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_READ, COUNT_WRITE, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE FROM performance_schema.table_io_waits_summary_by_table WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema')")
	if err != nil {
		log.Println("error running status query on database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
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
		// get RawBytes from data
		err = perfSchemaTableWaitsRows.Scan(&objectSchema, &objectName, &countRead, &countWrite, &countFetch, &countInsert, &countUpdate, &countDelete)
		if err != nil {
			log.Println("error getting result set:", err)
			return
		}

		// Set COUNT_READ
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|read", objectSchema, objectName),
			value:  string(countRead),
		}
		// Set COUNT_WRITE
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|write", objectSchema, objectName),
			value:  string(countWrite),
		}
		// Set COUNT_FETCH
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|fetch", objectSchema, objectName),
			value:  string(countFetch),
		}
		// Set COUNT_INSERT
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|insert", objectSchema, objectName),
			value:  string(countInsert),
		}
		// Set COUNT_UPDATE
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|update", objectSchema, objectName),
			value:  string(countUpdate),
		}
		// Set COUNT_DELETE
		scrapes <- metric{
			source: performanceSchema,
			key:    fmt.Sprintf("table_io_waits|%s.%s|delete", objectSchema, objectName),
			value:  string(countDelete),
		}
	}

	e.error.Set(0)
	e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
}

func (e *Exporter) setGenericMetric(subsystem string, name string, value float64) {
	if _, ok := e.metrics[name]; !ok {
		e.metrics[name] = prometheus.NewUntyped(prometheus.UntypedOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
		})
	}

	e.metrics[name].Set(value)
}

var globalStatusRegexp = regexp.MustCompile(`^(com|connection_errors|innodb_rows|performance_schema)_(.*)$`)

func (e *Exporter) setGlobalStatusMetric(name string, value float64) {
	match := globalStatusRegexp.FindStringSubmatch(name)
	if match == nil {
		e.setGenericMetric(globalStatus, name, value)
		return
	}
	switch match[1] {
	case "com":
		e.commands.With(prometheus.Labels{"command": match[2]}).Set(value)
	case "connection_errors":
		e.connectionErrors.With(prometheus.Labels{"error": match[2]}).Set(value)
	case "innodb_rows":
		e.innodbRows.With(prometheus.Labels{"operation": match[2]}).Set(value)
	case "performance_schema":
		e.globalPerformanceSchema.With(prometheus.Labels{"instrumentation": match[2]}).Set(value)
	}
}

var performanceSchemaRegexp = regexp.MustCompile(`^(.*)|(.*)|(.*)$`)

func (e *Exporter) setPerformanceSchemaMetric(name string, value float64) {
	match := performanceSchemaRegexp.FindStringSubmatch(name)
	if match == nil {
		return
	}
	switch match[1] {
	case "table_io_waits":
		objectSchemaName := strings.Split(match[2], ".")

		e.performanceSchemaTableWaits.With(
			prometheus.Labels{"schema": objectSchemaName[1], "name": objectSchemaName[2], "operation": match[3]},
		).Set(value)
	}
}

func (e *Exporter) setMetrics(scrapes <-chan metric) {
	for m := range scrapes {
		name := strings.ToLower(m.key)
		value, err := strconv.ParseFloat(m.value, 64)
		if err != nil {
			// convert/serve text values here ?
			continue
		}

		switch m.source {
		case slaveStatus:
			e.setGenericMetric(m.source, name, value)
		case globalStatus:
			e.setGlobalStatusMetric(name, value)
		case performanceSchema:
			e.setPerformanceSchemaMetric(name, value)
		}
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range e.metrics {
		m.Collect(metrics)
	}
}

func parseStatus(data []byte) string {
	logRexp := regexp.MustCompile(`\.([0-9]+$)`)
	logNum := logRexp.Find(data)

	switch {

	case string(data) == "Yes":
		return "1"

	case string(data) == "No":
		return "0"

	case len(logNum) > 1:
		return string(logNum[1:])

	default:
		return string(data)

	}
}

func main() {
	flag.Parse()

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if len(dsn) == 0 {
		log.Fatal("couldn't find environment variable DATA_SOURCE_NAME")
	}

	exporter := NewMySQLExporter(dsn)
	prometheus.MustRegister(exporter)
	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
<head><title>MySQLd exporter</title></head>
<body>
<h1>MySQLd exporter</h1>
<p><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>
`))
	})

	log.Infof("Starting Server: %s", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
