package main

import (
	"database/sql"
	"flag"
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
	namespace    = "mysql"
	slaveStatus  = "slave_status"
	globalStatus = "global_status"
)

var (
	listenAddress = flag.String("web.listen-address", ":9104", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	authUser      = flag.String("auth.user", "", "Username for basic auth.")
	authPass      = flag.String("auth.pass", "", "Password for basic auth.")
)

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
	return
}

type Exporter struct {
	dsn               string
	mutex             sync.RWMutex
	duration, error   prometheus.Gauge
	totalScrapes      prometheus.Counter
	metrics           map[string]prometheus.Gauge
	commands          *prometheus.CounterVec
	connectionErrors  *prometheus.CounterVec
	innodbRows        *prometheus.CounterVec
	performanceSchema *prometheus.CounterVec
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
		performanceSchema: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: globalStatus,
			Name:      "performance_schema_total",
			Help:      "Mysql instrumentations that could not be loaded or created due to memory constraints",
		}, []string{"instrumentation"}),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.metrics {
		m.Describe(ch)
	}

	e.commands.Describe(ch)
	e.connectionErrors.Describe(ch)
	e.innodbRows.Describe(ch)
	e.performanceSchema.Describe(ch)

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
	e.performanceSchema.Collect(ch)
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

	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Println("error opening connection to database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer db.Close()

	// fetch database status
	rows, err := db.Query("SHOW GLOBAL STATUS")
	if err != nil {
		log.Println("error running status query on database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer rows.Close()

	var key, val []byte

	for rows.Next() {
		// get RawBytes from data
		err = rows.Scan(&key, &val)
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
	rows, err = db.Query("SHOW SLAVE STATUS")
	if err != nil {
		log.Println("error running show slave query on database:", err)
		e.error.Set(1)
		e.duration.Set(float64(time.Now().UnixNano()-now) / 1000000000)
		return
	}
	defer rows.Close()

	var slaveCols []string

	slaveCols, err = rows.Columns()
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

	for rows.Next() {

		err = rows.Scan(scanArgs...)
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
		e.performanceSchema.With(prometheus.Labels{"instrumentation": match[2]}).Set(value)
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

	handler := prometheus.Handler()
	if *authUser != "" || *authPass != "" {
		if *authUser == "" || *authPass == "" {
			log.Fatal("You need to specify -auth.user and -auth.pass to enable basic auth")
		}
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			user:     *authUser,
			password: *authPass,
		}
	}

	http.Handle(*metricPath, handler)
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
