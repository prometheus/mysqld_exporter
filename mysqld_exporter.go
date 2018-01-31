package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"

	"github.com/percona/mysqld_exporter/collector"
)

var (
	listenAddress = kingpin.Flag(
		"web.listen-address",
		"Address to listen on for web interface and telemetry.",
	).Default(":9104").String()
	metricPath = kingpin.Flag(
		"web.telemetry-path",
		"Path under which to expose metrics.",
	).Default("/metrics").String()
	configMycnf = kingpin.Flag(
		"config.my-cnf",
		"Path to .my.cnf file to read MySQL credentials from.",
	).Default(path.Join(os.Getenv("HOME"), ".my.cnf")).String()
	webAuthFile = kingpin.Flag(
		"web.auth-file",
		"Path to YAML file with server_user, server_password options for http basic auth (overrides HTTP_AUTH env var).",
	).String()
	sslCertFile = kingpin.Flag(
		"web.ssl-cert-file",
		"Path to SSL certificate file.",
	).String()
	sslKeyFile = kingpin.Flag(
		"web.ssl-key-file",
		"Path to SSL key file.",
	).String()
	collectProcesslist = kingpin.Flag(
		"collect.info_schema.processlist",
		"Collect current thread state counts from the information_schema.processlist",
	).Default("false").Bool()
	collectTableSchema = kingpin.Flag(
		"collect.info_schema.tables",
		"Collect metrics from information_schema.tables",
	).Default("false").Bool()
	collectInnodbTablespaces = kingpin.Flag(
		"collect.info_schema.innodb_tablespaces",
		"Collect metrics from information_schema.innodb_sys_tablespaces",
	).Default("false").Bool()
	collectInnodbMetrics = kingpin.Flag(
		"collect.info_schema.innodb_metrics",
		"Collect metrics from information_schema.innodb_metrics",
	).Default("false").Bool()
	collectGlobalStatus = kingpin.Flag(
		"collect.global_status",
		"Collect from SHOW GLOBAL STATUS",
	).Default("false").Bool()
	collectGlobalVariables = kingpin.Flag(
		"collect.global_variables",
		"Collect from SHOW GLOBAL VARIABLES",
	).Default("false").Bool()
	collectSlaveStatus = kingpin.Flag(
		"collect.slave_status",
		"Collect from SHOW SLAVE STATUS",
	).Default("false").Bool()
	collectAutoIncrementColumns = kingpin.Flag(
		"collect.auto_increment.columns",
		"Collect auto_increment columns and max values from information_schema",
	).Default("false").Bool()
	collectBinlogSize = kingpin.Flag(
		"collect.binlog_size",
		"Collect the current size of all registered binlog files",
	).Default("false").Bool()
	collectPerfTableIOWaits = kingpin.Flag(
		"collect.perf_schema.tableiowaits",
		"Collect metrics from performance_schema.table_io_waits_summary_by_table",
	).Default("false").Bool()
	collectPerfIndexIOWaits = kingpin.Flag(
		"collect.perf_schema.indexiowaits",
		"Collect metrics from performance_schema.table_io_waits_summary_by_index_usage",
	).Default("false").Bool()
	collectPerfTableLockWaits = kingpin.Flag(
		"collect.perf_schema.tablelocks",
		"Collect metrics from performance_schema.table_lock_waits_summary_by_table",
	).Default("false").Bool()
	collectPerfEventsStatements = kingpin.Flag(
		"collect.perf_schema.eventsstatements",
		"Collect metrics from performance_schema.events_statements_summary_by_digest",
	).Default("false").Bool()
	collectPerfEventsWaits = kingpin.Flag(
		"collect.perf_schema.eventswaits",
		"Collect metrics from performance_schema.events_waits_summary_global_by_event_name",
	).Default("false").Bool()
	collectPerfFileEvents = kingpin.Flag(
		"collect.perf_schema.file_events",
		"Collect metrics from performance_schema.file_summary_by_event_name",
	).Default("false").Bool()
	collectPerfFileInstances = kingpin.Flag(
		"collect.perf_schema.file_instances",
		"Collect metrics from performance_schema.file_summary_by_instance",
	).Default("false").Bool()
	collectUserStat = kingpin.Flag(
		"collect.info_schema.userstats",
		"If running with userstat=1, set to true to collect user statistics",
	).Default("false").Bool()
	collectClientStat = kingpin.Flag(
		"collect.info_schema.clientstats",
		"If running with userstat=1, set to true to collect client statistics",
	).Default("false").Bool()
	collectTableStat = kingpin.Flag(
		"collect.info_schema.tablestats",
		"If running with userstat=1, set to true to collect table statistics",
	).Default("false").Bool()
	collectQueryResponseTime = kingpin.Flag(
		"collect.info_schema.query_response_time",
		"Collect query response time distribution if query_response_time_stats is ON.",
	).Default("false").Bool()
	collectEngineTokudbStatus = kingpin.Flag("collect.engine_tokudb_status",
		"Collect from SHOW ENGINE TOKUDB STATUS",
	).Default("false").Bool()
	collectEngineInnodbStatus = kingpin.Flag(
		"collect.engine_innodb_status",
		"Collect from SHOW ENGINE INNODB STATUS",
	).Default("false").Bool()
	collectHeartbeat = kingpin.Flag(
		"collect.heartbeat",
		"Collect from heartbeat",
	).Default("false").Bool()
	collectHeartbeatDatabase = kingpin.Flag(
		"collect.heartbeat.database",
		"Database from where to collect heartbeat data",
	).Default("heartbeat").String()
	collectHeartbeatTable = kingpin.Flag(
		"collect.heartbeat.table",
		"Table from where to collect heartbeat data",
	).Default("heartbeat").String()
	dsn string
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

func newHandler(cfg *webAuth, scrapers []collector.Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var filters map[string]bool
		params := r.URL.Query()["collect[]"]
		log.Debugln("collect query:", params)

		if len(params) > 0 {
			filters = make(map[string]bool)
			for _, param := range params {
				filters[param] = true
			}

			var filteredScrapers []collector.Scraper
			for _, scraper := range scrapers {
				if filters[scraper.Name()] {
					filteredScrapers = append(filteredScrapers, scraper)
				}
			}
			scrapers = filteredScrapers
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(dsn, scrapers))

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}
		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		if cfg.User != "" && cfg.Password != "" {
			h = &basicAuthHandler{handler: h.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		h.ServeHTTP(w, r)
	}

}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// landingPage contains the HTML served at '/'.
	var landingPage = []byte(`<html>
<head><title>MySQLd 3-in-1 exporter</title></head>
<body>
<h1>MySQL 3-in-1 exporter</h1>
<li><a href="` + *metricPath + `-hr">high-res metrics</a></li>
<li><a href="` + *metricPath + `-mr">medium-res metrics</a></li>
<li><a href="` + *metricPath + `-lr">low-res metrics</a></li>
</body>
</html>
`)

	log.Infoln("Starting mysqld_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	dsn = os.Getenv("DATA_SOURCE_NAME")
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

	// New http server
	mux := http.NewServeMux()

	// Defines what to scrape in high resolution.
	scrapersHr := Scrapers{
		{collector.ScraperGlobalStatus, *collectGlobalStatus},
		{collector.ScraperInnodbMetrics, *collectInnodbMetrics},
	}
	mux.Handle(*metricPath+"-hr", newHandler(
		cfg,
		scrapersHr.CollectorActiveScrapers(),
	))

	// Defines what to scrape in medium resolution.
	scrapersMr := Scrapers{
		{collector.ScraperSlaveStatus, *collectSlaveStatus},
		{collector.ScraperProcessList, *collectProcesslist},
		{collector.ScraperPerfEventsWaits, *collectPerfEventsWaits},
		{collector.ScraperPerfFileEvents, *collectPerfFileEvents},
		{collector.ScraperPerfTableLockWaits, *collectPerfTableLockWaits},
		{collector.ScraperQueryResponseTime, *collectQueryResponseTime},
		{collector.ScraperEngineInnodbStatus, *collectEngineInnodbStatus},
	}
	mux.Handle(*metricPath+"-mr", newHandler(
		cfg,
		scrapersMr.CollectorActiveScrapers(),
	))

	// Defines what to scrape in low resolution.
	scrapersLr := Scrapers{
		{collector.ScraperGlobalVariables, *collectGlobalVariables},
		{collector.ScraperTableSchema, *collectTableSchema},
		{collector.ScraperAutoIncrementColumns, *collectAutoIncrementColumns},
		{collector.ScraperBinlogSize, *collectBinlogSize},
		{collector.ScraperPerfTableIOWaits, *collectPerfTableIOWaits},
		{collector.ScraperPerfIndexIOWaits, *collectPerfIndexIOWaits},
		{collector.ScraperPerfFileInstances, *collectPerfFileInstances},
		{collector.ScraperUserStat, *collectUserStat},
		{collector.ScraperTableStat, *collectTableStat},
		{collector.ScraperPerfEventsStatements, *collectPerfEventsStatements},
		{collector.ScraperClientStat, *collectClientStat},
		{collector.ScraperInfoSchemaInnodbTablespaces, *collectInnodbTablespaces},
		{collector.ScraperEngineTokudbStatus, *collectEngineTokudbStatus},
		{collector.ScraperHeartbeat(*collectHeartbeatDatabase, *collectHeartbeatTable), *collectHeartbeat},
	}
	mux.Handle(*metricPath+"-lr", newHandler(
		cfg,
		scrapersLr.CollectorActiveScrapers(),
	))

	srv := &http.Server{
		Addr:    *listenAddress,
		Handler: mux,
	}

	log.Infoln("Listening on", *listenAddress)
	if ssl {
		// https
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
		srv.TLSConfig = tlsCfg
		srv.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0)

		log.Fatal(srv.ListenAndServeTLS(*sslCertFile, *sslKeyFile))
	} else {
		// http
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage)
		})

		log.Fatal(srv.ListenAndServe())
	}
}

type Scraper struct {
	scraper collector.Scraper
	on      bool
}

type Scrapers []Scraper

func (s Scrapers) CollectorActiveScrapers() []collector.Scraper {
	var active []collector.Scraper
	for _, scraper := range s {
		if scraper.on {
			active = append(active, scraper.scraper)
		}

	}

	return active
}
