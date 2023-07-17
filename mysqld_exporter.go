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
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/prometheus/mysqld_exporter/collector"
	"github.com/prometheus/mysqld_exporter/config"
)

var (
	metricsPath = kingpin.Flag(
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
	).Default(".my.cnf").String()
	mysqldAddress = kingpin.Flag(
		"mysqld.address",
		"Address to use for connecting to MySQL",
	).Default("localhost:3306").String()
	mysqldUser = kingpin.Flag(
		"mysqld.username",
		"Hostname to use for connecting to MySQL",
	).String()
	tlsInsecureSkipVerify = kingpin.Flag(
		"tls.insecure-skip-verify",
		"Ignore certificate and server verification when using a tls connection.",
	).Bool()
	toolkitFlags = webflag.AddFlags(kingpin.CommandLine, ":9104")
	c            = config.MySqlConfigHandler{
		Config: &config.Config{},
	}
)

// scrapers lists all possible collection methods and if they should be enabled by default.
var scrapers = map[collector.Scraper]bool{
	collector.ScrapeGlobalStatus{}:                        true,
	collector.ScrapeGlobalVariables{}:                     true,
	collector.ScrapeSlaveStatus{}:                         true,
	collector.ScrapeProcesslist{}:                         false,
	collector.ScrapeUser{}:                                false,
	collector.ScrapeTableSchema{}:                         false,
	collector.ScrapeInfoSchemaInnodbTablespaces{}:         false,
	collector.ScrapeInnodbMetrics{}:                       false,
	collector.ScrapeAutoIncrementColumns{}:                false,
	collector.ScrapeBinlogSize{}:                          false,
	collector.ScrapePerfTableIOWaits{}:                    false,
	collector.ScrapePerfIndexIOWaits{}:                    false,
	collector.ScrapePerfTableLockWaits{}:                  false,
	collector.ScrapePerfEventsStatements{}:                false,
	collector.ScrapePerfEventsStatementsSum{}:             false,
	collector.ScrapePerfEventsWaits{}:                     false,
	collector.ScrapePerfFileEvents{}:                      false,
	collector.ScrapePerfFileInstances{}:                   false,
	collector.ScrapePerfMemoryEvents{}:                    false,
	collector.ScrapePerfReplicationGroupMembers{}:         false,
	collector.ScrapePerfReplicationGroupMemberStats{}:     false,
	collector.ScrapePerfReplicationApplierStatsByWorker{}: false,
	collector.ScrapeSysUserSummary{}:                      false,
	collector.ScrapeUserStat{}:                            false,
	collector.ScrapeClientStat{}:                          false,
	collector.ScrapeTableStat{}:                           false,
	collector.ScrapeSchemaStat{}:                          false,
	collector.ScrapeInnodbCmp{}:                           true,
	collector.ScrapeInnodbCmpMem{}:                        true,
	collector.ScrapeQueryResponseTime{}:                   true,
	collector.ScrapeEngineTokudbStatus{}:                  false,
	collector.ScrapeEngineInnodbStatus{}:                  false,
	collector.ScrapeHeartbeat{}:                           false,
	collector.ScrapeSlaveHosts{}:                          false,
	collector.ScrapeReplicaHost{}:                         false,
}

func filterScrapers(scrapers []collector.Scraper, collectParams []string) []collector.Scraper {
	var filteredScrapers []collector.Scraper

	// Check if we have some "collect[]" query parameters.
	if len(collectParams) > 0 {
		filters := make(map[string]bool)
		for _, param := range collectParams {
			filters[param] = true
		}

		for _, scraper := range scrapers {
			if filters[scraper.Name()] {
				filteredScrapers = append(filteredScrapers, scraper)
			}
		}
	}
	if len(filteredScrapers) == 0 {
		return scrapers
	}
	return filteredScrapers
}

func init() {
	prometheus.MustRegister(version.NewCollector("mysqld_exporter"))
}

func newHandler(scrapers []collector.Scraper, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dsn string
		var err error
		target := ""
		q := r.URL.Query()
		if q.Has("target") {
			target = q.Get("target")
		}

		cfg := c.GetConfig()
		cfgsection, ok := cfg.Sections["client"]
		if !ok {
			level.Error(logger).Log("msg", "Failed to parse section [client] from config file", "err", err)
		}
		if dsn, err = cfgsection.FormDSN(target); err != nil {
			level.Error(logger).Log("msg", "Failed to form dsn from section [client]", "err", err)
		}

		collect := q["collect[]"]

		// Use request context for cancellation when connection gets closed.
		ctx := r.Context()
		// If a timeout is configured via the Prometheus header, add it to the context.
		if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
			timeoutSeconds, err := strconv.ParseFloat(v, 64)
			if err != nil {
				level.Error(logger).Log("msg", "Failed to parse timeout from Prometheus header", "err", err)
			} else {
				if *timeoutOffset >= timeoutSeconds {
					// Ignore timeout offset if it doesn't leave time to scrape.
					level.Error(logger).Log("msg", "Timeout offset should be lower than prometheus scrape timeout", "offset", *timeoutOffset, "prometheus_scrape_timeout", timeoutSeconds)
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

		filteredScrapers := filterScrapers(scrapers, collect)

		registry := prometheus.NewRegistry()

		registry.MustRegister(collector.New(ctx, dsn, filteredScrapers, logger))

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
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting mysqld_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	var err error
	if err = c.ReloadConfig(*configMycnf, *mysqldAddress, *mysqldUser, *tlsInsecureSkipVerify, logger); err != nil {
		level.Info(logger).Log("msg", "Error parsing host config", "file", *configMycnf, "err", err)
		os.Exit(1)
	}

	// Register only scrapers enabled by flag.
	enabledScrapers := []collector.Scraper{}
	for scraper, enabled := range scraperFlags {
		if *enabled {
			level.Info(logger).Log("msg", "Scraper enabled", "scraper", scraper.Name())
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}
	handlerFunc := newHandler(enabledScrapers, logger)
	http.Handle(*metricsPath, promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, handlerFunc))
	if *metricsPath != "/" && *metricsPath != "" {
		landingConfig := web.LandingConfig{
			Name:        "MySQLd Exporter",
			Description: "Prometheus Exporter for MySQL servers",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			level.Error(logger).Log("err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}
	http.HandleFunc("/probe", handleProbe(enabledScrapers, logger))
	http.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		if err = c.ReloadConfig(*configMycnf, *mysqldAddress, *mysqldUser, *tlsInsecureSkipVerify, logger); err != nil {
			level.Warn(logger).Log("msg", "Error reloading host config", "file", *configMycnf, "error", err)
			return
		}
		_, _ = w.Write([]byte(`ok`))
	})
	srv := &http.Server{}
	if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
