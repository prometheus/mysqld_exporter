package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "mysql"
)

var (
	listenAddress = flag.String("web.listen-address", ":9091", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	configFile    = flag.String("config.file", "mysqld_exporter.conf", "Path to config file.")
)

type Config struct {
	Config map[string]string `json:"config"`
}

type Exporter struct {
	config                     Config
	mutex                      sync.RWMutex
	up                         prometheus.Gauge
	totalScrapes, errorScrapes prometheus.Counter
	metrics                    map[string]prometheus.Gauge
}

func getConfig(file string) (*Config, error) {
	config := &Config{}

	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return config, json.Unmarshal(bytes, &config)
}

// return new empty exporter
func NewMySQLExporter(config *Config) *Exporter {
	return &Exporter{
		config: *config,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of mysqld successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_scrapes",
			Help:      "Current total mysqld scrapes.",
		}),
		errorScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_error_scrapes",
			Help:      "Error mysqld scrapes.",
		}),
		metrics: map[string]prometheus.Gauge{},
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.metrics {
		m.Describe(ch)
	}

	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.errorScrapes.Desc()
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	scrapes := make(chan []string)

	go e.scrape(scrapes)

	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.setMetrics(scrapes)
	ch <- e.up
	ch <- e.totalScrapes
	ch <- e.errorScrapes
	e.collectMetrics(ch)
}

func (e *Exporter) scrape(scrapes chan<- []string) {
	defer close(scrapes)

	e.totalScrapes.Inc()
	e.up.Set(0)

	db, err := sql.Open("mysql", e.config.Config["mysql_connection"])
	if err != nil {
		log.Printf("error open connection to db using %s", e.config.Config["mysql_connection"])
		return
	}
	defer db.Close()

	rows, err := db.Query("SHOW STATUS")
	if err != nil {
		log.Printf("error running query on db %s", e.config.Config["mysql_connection"])
		return
	}
	defer rows.Close()

	e.up.Set(1) // from this point db is ok

	var key, val []byte
	for rows.Next() {
		// get RawBytes from data
		err = rows.Scan(&key, &val)
		if err != nil {
			log.Printf("error getting result set ")
			return
		}

		var res []string = make([]string, 2)
		res[0] = string(key)
		res[1] = string(val)

		scrapes <- res
	}

	return
}

func (e *Exporter) setMetrics(scrapes <-chan []string) {
	for row := range scrapes {

		name := row[0]
		value, err := strconv.ParseInt(row[1], 10, 64)
		if err != nil {
			// not really important
			// log.Printf("Error while parsing field value %s (%s): %v", row[0], row[1], err)
			e.errorScrapes.Inc()
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

	config, err := getConfig(*configFile)
	if err != nil {
		log.Fatal("couldn't read config file ", *configFile, " : ", err)
	}

	exporter := NewMySQLExporter(config)
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
