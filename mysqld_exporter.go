// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"

	"github.com/prometheus/mysqld_exporter/collector"
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
	timeoutOffset = kingpin.Flag(
		"timeout-offset",
		"Offset to subtract from timeout in seconds.",
	).Default("0.25").Float64()
	configMycnf = kingpin.Flag(
		"config.my-cnf",
		"Path to .my.cnf file to read MySQL credentials from.",
	).Default(path.Join(os.Getenv("HOME"), ".my.cnf")).String()
	dsn string
)

// scrapers lists all possible collection methods and if they should be enabled by default.
var scrapers = map[collector.Scraper]bool{
	collector.ScrapeGlobalStatus{}:                    true,
	collector.ScrapeGlobalVariables{}:                 true,
	collector.ScrapeSlaveStatus{}:                     true,
	collector.ScrapeProcesslist{}:                     false,
	collector.ScrapeUser{}:                            false,
	collector.ScrapeTableSchema{}:                     true,
	collector.ScrapeInfoSchemaInnodbTablespaces{}:     false,
	collector.ScrapeInnodbMetrics{}:                   false,
	collector.ScrapeAutoIncrementColumns{}:            false,
	collector.ScrapeBinlogSize{}:                      false,
	collector.ScrapePerfTableIOWaits{}:                false,
	collector.ScrapePerfIndexIOWaits{}:                false,
	collector.ScrapePerfTableLockWaits{}:              false,
	collector.ScrapePerfEventsStatements{}:            false,
	collector.ScrapePerfEventsWaits{}:                 false,
	collector.ScrapePerfFileEvents{}:                  false,
	collector.ScrapePerfFileInstances{}:               false,
	collector.ScrapePerfReplicationGroupMemberStats{}: false,
	collector.ScrapeUserStat{}:                        false,
	collector.ScrapeClientStat{}:                      false,
	collector.ScrapeTableStat{}:                       false,
	collector.ScrapeInnodbCmp{}:                       true,
	collector.ScrapeInnodbCmpMem{}:                    true,
	collector.ScrapeQueryResponseTime{}:               true,
	collector.ScrapeEngineTokudbStatus{}:              false,
	collector.ScrapeEngineInnodbStatus{}:              false,
	collector.ScrapeHeartbeat{}:                       false,
	collector.ScrapeSlaveHosts{}:                      false,
}

func parseMycnf(config interface{}) (string, error) {
	var dsn string
	opts := ini.LoadOptions{
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
	}
	cfg, err := ini.LoadSources(opts, config)
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
	sslCA := cfg.Section("client").Key("ssl-ca").String()
	sslCert := cfg.Section("client").Key("ssl-cert").String()
	sslKey := cfg.Section("client").Key("ssl-key").String()
	if sslCA != "" {
		if tlsErr := customizeTLS(sslCA, sslCert, sslKey); tlsErr != nil {
			tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
			return dsn, tlsErr
		}
		dsn = fmt.Sprintf("%s?tls=custom", dsn)
	}

	log.Debugln(dsn)
	return dsn, nil
}

func customizeTLS(sslCA string, sslCert string, sslKey string) error {
	var tlsCfg tls.Config
	caBundle := x509.NewCertPool()
	pemCA, err := ioutil.ReadFile(sslCA)
	if err != nil {
		return err
	}
	if ok := caBundle.AppendCertsFromPEM(pemCA); ok {
		tlsCfg.RootCAs = caBundle
	} else {
		return fmt.Errorf("failed parse pem-encoded CA certificates from %s", sslCA)
	}
	if sslCert != "" && sslKey != "" {
		certPairs := make([]tls.Certificate, 0, 1)
		keypair, err := tls.LoadX509KeyPair(sslCert, sslKey)
		if err != nil {
			return fmt.Errorf("failed to parse pem-encoded SSL cert %s or SSL key %s: %s",
				sslCert, sslKey, err)
		}
		certPairs = append(certPairs, keypair)
		tlsCfg.Certificates = certPairs
	}
	mysql.RegisterTLSConfig("custom", &tlsCfg)
	return nil
}

func init() {
	prometheus.MustRegister(version.NewCollector("mysqld_exporter"))
}

func newHandler(metrics collector.Metrics, scrapers []collector.Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filteredScrapers := scrapers
		params := r.URL.Query()["collect[]"]
		// Use request context for cancellation when connection gets closed.
		ctx := r.Context()
		// If a timeout is configured via the Prometheus header, add it to the context.
		if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
			timeoutSeconds, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Errorf("Failed to parse timeout from Prometheus header: %s", err)
			} else {
				if *timeoutOffset >= timeoutSeconds {
					// Ignore timeout offset if it doesn't leave time to scrape.
					log.Errorf(
						"Timeout offset (--timeout-offset=%.2f) should be lower than prometheus scrape time (X-Prometheus-Scrape-Timeout-Seconds=%.2f).",
						*timeoutOffset,
						timeoutSeconds,
					)
				} else {
					// Subtract timeout offset from timeout.
					timeoutSeconds -= *timeoutOffset
				}
				// Create new timeout context with request context as parent.
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds*float64(time.Second)))
				defer cancel()
				// Overwrite request with timeout context.
				r = r.WithContext(ctx)
			}
		}
		log.Debugln("collect query:", params)

		// Check if we have some "collect[]" query parameters.
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
		registry.MustRegister(collector.New(ctx, dsn, metrics, filteredScrapers))

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}
		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

func main() {
	// Generate ON/OFF flags for all scrapers.
	scraperFlags := map[collector.Scraper]*bool{}
	for scraper, enabledByDefault := range scrapers {
		defaultOn := "false"
		if enabledByDefault {
			defaultOn = "true"
		}

		f := kingpin.Flag(
			"collect."+scraper.Name(),
			scraper.Help(),
		).Default(defaultOn).Bool()

		scraperFlags[scraper] = f
	}

	// Parse flags.
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

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

	log.Infoln("Starting mysqld_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	dsn = os.Getenv("DATA_SOURCE_NAME")
	if len(dsn) == 0 {
		var err error
		if dsn, err = parseMycnf(*configMycnf); err != nil {
			log.Fatal(err)
		}
	}

	// Register only scrapers enabled by flag.
	log.Infof("Enabled scrapers:")
	enabledScrapers := []collector.Scraper{}
	for scraper, enabled := range scraperFlags {
		if *enabled {
			log.Infof(" --collect.%s", scraper.Name())
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}
	handlerFunc := newHandler(collector.NewMetrics(), enabledScrapers)
	http.HandleFunc(*metricPath, prometheus.InstrumentHandlerFunc("metrics", handlerFunc))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
