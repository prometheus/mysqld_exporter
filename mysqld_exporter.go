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
	configPath = kingpin.Flag(
		"config",
		"Path to YAML file containing configuration.",
	).String()
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
)

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

func newHandler(scrapers []collector.Scraper, configFn func() *config.Config, mycnfFn func() config.Mycnf, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dsn string
		var err error
		target := ""
		q := r.URL.Query()
		if q.Has("target") {
			target = q.Get("target")
		}

		mycnf := mycnfFn()
		cfgsection, ok := mycnf["client"]
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
	// Parse flags.
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting mysqld_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	// Get a list of all scrapers
	scrapers := []collector.Scraper{}
	for _, scraper := range collector.AllScrapers() {
		scrapers = append(scrapers, scraper)
	}

	// Set up config reloaders.
	configReloader := config.NewConfigReloader(func() (*config.Config, error) {
		newConfig := &config.Config{}

		newConfig.Merge(config.FromDefaults())

		configFromFlags, err := config.FromFlags()
		if err != nil {
			return nil, err
		}
		newConfig.Merge(configFromFlags)

		var configFromFile *config.Config
		if *configPath != "" {
			var err error
			configFromFile, err = config.FromFile(*configPath)
			if err != nil {
				return nil, err
			}
			newConfig.Merge(configFromFile)
		}

		return newConfig, nil
	})
	if err := configReloader.Reload(); err != nil {
		level.Info(logger).Log("msg", "Error parsing host config", "file", *configPath, "err", err)
		os.Exit(1)
	}
	config.Apply(configReloader.Config())

	mycnfReloader := config.NewMycnfReloader(&config.MycnfReloaderOpts{
		MycnfPath:                    *configMycnf,
		DefaultMysqldAddress:         *mysqldAddress,
		DefaultMysqldUser:            *mysqldUser,
		DefaultTlsInsecureSkipVerify: *tlsInsecureSkipVerify,
		Logger:                       logger,
	})
	if err := mycnfReloader.Reload(); err != nil {
		level.Info(logger).Log("msg", "Error parsing host config", "file", *configMycnf, "err", err)
		os.Exit(1)
	}

	// Report which scrapers are enabled.
	for _, collector := range configReloader.Config().Collectors {
		if collector.Enabled != nil && *collector.Enabled {
			level.Info(logger).Log("msg", "Scraper enabled", "scraper", collector.Name)
		}
	}

	handlerFunc := newHandler(scrapers, configReloader.Config, mycnfReloader.Mycnf, logger)
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
	http.HandleFunc("/probe", handleProbe(scrapers, mycnfReloader.Mycnf, logger))
	http.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		// Reload configuration.
		if err := configReloader.Reload(); err != nil {
			level.Warn(logger).Log("msg", "Error reloading host config", "file", *configPath, "error", err)
			return
		}

		// Configure registered collectors with latest configuration.
		config.Apply(configReloader.Config())

		// Reload mycnf file.
		if err := mycnfReloader.Reload(); err != nil {
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
