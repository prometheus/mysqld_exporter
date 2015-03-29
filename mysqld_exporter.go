package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "mysql"
)

var (
	listenAddress = flag.String("web.listen-address", ":9104", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

type Exporter struct {
	dsn                        string
	mutex                      sync.RWMutex
	duration                   prometheus.Gauge
	totalScrapes, errorScrapes prometheus.Counter
	metrics                    map[string]prometheus.Gauge
}

// return new empty exporter
func NewMySQLExporter(dsn string) *Exporter {
	return &Exporter{
		dsn: dsn,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "exporter_last_scrape_duration_nanoseconds",
			Help:      "Was the last scrape  duration.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total mysqld scrapes.",
		}),
		errorScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_errors_total",
			Help:      "Error mysqld scrapes.",
		}),
		metrics: map[string]prometheus.Gauge{},
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.metrics {
		m.Describe(ch)
	}

	ch <- e.duration.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.errorScrapes.Desc()
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	scrapes := make(chan []string)

	go e.scrape(scrapes)

	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.setMetrics(scrapes)
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.errorScrapes
	e.collectMetrics(ch)
}

func (e *Exporter) scrape(scrapes chan<- []string) {
	defer close(scrapes)

	now := time.Now().UnixNano()

	e.totalScrapes.Inc()

	db, err := sql.Open("mysql", e.dsn)
	if err != nil {
		log.Printf("error open connection to db")
		e.errorScrapes.Inc()
		e.duration.Set(float64(time.Now().UnixNano() - now))
		return
	}
	defer db.Close()

	rows, err := db.Query("SHOW STATUS")
	if err != nil {
		log.Printf("error running query on db")
		e.errorScrapes.Inc()
		e.duration.Set(float64(time.Now().UnixNano() - now))
		return
	}
	defer rows.Close()

	var key, val []byte
	for rows.Next() {
		// get RawBytes from data
		err = rows.Scan(&key, &val)
		if err != nil {
			log.Printf("error getting result set")
			return
		}

		var res []string = make([]string, 2)
		res[0] = string(key)
		res[1] = string(val)

		scrapes <- res
	}

	e.duration.Set(float64(time.Now().UnixNano() - now))
}

func (e *Exporter) setMetrics(scrapes <-chan []string) {
	for row := range scrapes {

		name := strings.ToLower(row[0])
		value, err := strconv.ParseInt(row[1], 10, 64)
		if err != nil {
			// convert/serve text values here ?
			continue
		}

		if _, ok := e.metrics[name]; !ok {
			e.metrics[name] = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      name,
			})
		}

		e.metrics[name].Set(float64(value))
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range e.metrics {
		m.Collect(metrics)
	}
}

func main() {
	flag.Parse()

	dsn := os.Getenv("DSN")
	if len(dsn) == 0 {
		log.Fatal("couldn't find environment variable DSN")
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

	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
