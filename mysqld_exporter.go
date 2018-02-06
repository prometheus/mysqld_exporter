package main

import (
	"crypto/tls"
	"flag"
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

// scrapers lists all possible collection methods and if they should be enabled by default.
var scrapers = map[collector.Scraper]bool{
	collector.ScrapeGlobalStatus{}:                false,
	collector.ScrapeGlobalVariables{}:             false,
	collector.ScrapeSlaveStatus{}:                 false,
	collector.ScrapeProcesslist{}:                 false,
	collector.ScrapeTableSchema{}:                 false,
	collector.ScrapeInfoSchemaInnodbTablespaces{}: false,
	collector.ScrapeInnodbMetrics{}:               false,
	collector.ScrapeAutoIncrementColumns{}:        false,
	collector.ScrapeBinlogSize{}:                  false,
	collector.ScrapePerfTableIOWaits{}:            false,
	collector.ScrapePerfIndexIOWaits{}:            false,
	collector.ScrapePerfTableLockWaits{}:          false,
	collector.ScrapePerfEventsStatements{}:        false,
	collector.ScrapePerfEventsWaits{}:             false,
	collector.ScrapePerfFileEvents{}:              false,
	collector.ScrapePerfFileInstances{}:           false,
	collector.ScrapeUserStat{}:                    false,
	collector.ScrapeClientStat{}:                  false,
	collector.ScrapeTableStat{}:                   false,
	collector.ScrapeQueryResponseTime{}:           false,
	collector.ScrapeEngineTokudbStatus{}:          false,
	collector.ScrapeEngineInnodbStatus{}:          false,
	collector.ScrapeHeartbeat{}:                   false,
}

var scrapersHr = map[collector.Scraper]struct{}{
	collector.ScrapeGlobalStatus{}:  {},
	collector.ScrapeInnodbMetrics{}: {},
}

var scrapersMr = map[collector.Scraper]struct{}{
	collector.ScrapeSlaveStatus{}:        {},
	collector.ScrapeProcesslist{}:        {},
	collector.ScrapePerfEventsWaits{}:    {},
	collector.ScrapePerfFileEvents{}:     {},
	collector.ScrapePerfTableLockWaits{}: {},
	collector.ScrapeQueryResponseTime{}:  {},
	collector.ScrapeEngineInnodbStatus{}: {},
}

var scrapersLr = map[collector.Scraper]struct{}{
	collector.ScrapeGlobalVariables{}:             {},
	collector.ScrapeTableSchema{}:                 {},
	collector.ScrapeAutoIncrementColumns{}:        {},
	collector.ScrapeBinlogSize{}:                  {},
	collector.ScrapePerfTableIOWaits{}:            {},
	collector.ScrapePerfIndexIOWaits{}:            {},
	collector.ScrapePerfFileInstances{}:           {},
	collector.ScrapeUserStat{}:                    {},
	collector.ScrapeTableStat{}:                   {},
	collector.ScrapePerfEventsStatements{}:        {},
	collector.ScrapeClientStat{}:                  {},
	collector.ScrapeInfoSchemaInnodbTablespaces{}: {},
	collector.ScrapeEngineTokudbStatus{}:          {},
	collector.ScrapeHeartbeat{}:                   {},
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
		filteredScrapers := scrapers
		params := r.URL.Query()["collect[]"]
		log.Debugln("collect query:", params)

		if len(params) > 0 {
			filters := make(map[string]bool)
			for _, param := range params {
				filters[param] = true
			}

			filteredScrapers = nil
			for _, scraper := range scrapers {
				if filters[scraper.Name()] {
					filteredScrapers = append(filteredScrapers, scraper)
				}
			}
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(dsn, filteredScrapers))

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}
		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			// mysqld_exporter has multiple collectors, if one fails,
			// we still should report metrics from collectors that succeeded.
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      log.NewErrorLogger(),
		})
		if cfg.User != "" && cfg.Password != "" {
			h = &basicAuthHandler{handler: h.ServeHTTP, user: cfg.User, password: cfg.Password}
		}
		h.ServeHTTP(w, r)
	}
}

func main() {
	// Generate ON/OFF flags for all scrapers.
	scraperFlags := map[collector.Scraper]*bool{}
	for scraper, enabledByDefault := range scrapers {
		f := flag.Bool(
			"collect."+scraper.Name(), enabledByDefault,
			scraper.Help(),
		)

		scraperFlags[scraper] = f
	}

	// Parse flags.
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("mysqld_exporter"))
		os.Exit(0)
	}

	// landingPage contains the HTML served at '/'.
	// TODO: Make this nicer and more informative.
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

	// Defines what to scrape in each resolution.
	hr, mr, lr := enabledScrapers(scraperFlags)
	mux.Handle(*metricPath+"-hr", newHandler(cfg, hr))
	mux.Handle(*metricPath+"-mr", newHandler(cfg, mr))
	mux.Handle(*metricPath+"-lr", newHandler(cfg, lr))

	// Log which scrapers are enabled.
	if len(hr) > 0 {
		log.Infof("Enabled High Resolution scrapers:")
		for _, scraper := range hr {
			log.Infof(" --collect.%s", scraper.Name())
		}
	}
	if len(mr) > 0 {
		log.Infof("Enabled Medium Resolution scrapers:")
		for _, scraper := range mr {
			log.Infof(" --collect.%s", scraper.Name())
		}
	}
	if len(lr) > 0 {
		log.Infof("Enabled Low Resolution scrapers:")
		for _, scraper := range lr {
			log.Infof(" --collect.%s", scraper.Name())
		}
	}

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

func enabledScrapers(scraperFlags map[collector.Scraper]*bool) (hr, mr, lr []collector.Scraper) {
	for scraper, enabled := range scraperFlags {
		if *enabled {
			if _, ok := scrapersHr[scraper]; ok {
				hr = append(hr, scraper)
			}
			if _, ok := scrapersMr[scraper]; ok {
				mr = append(mr, scraper)
			}
			if _, ok := scrapersLr[scraper]; ok {
				lr = append(lr, scraper)
			}
		}
	}

	return hr, mr, lr
}
