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
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
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
	exporterLockTimeout = kingpin.Flag(
		"exporter.lock_wait_timeout",
		"Set a lock_wait_timeout (in seconds) on the connection to avoid long metadata locking.",
	).Default("2").Int()
	enableExporterLockTimeout = kingpin.Flag(
		"exporter.enable_lock_wait_timeout",
		"Enable the lock_wait_timeout MySQL connection parameter.",
	).Default("true").Bool()
	slowLogFilter = kingpin.Flag(
		"exporter.log_slow_filter",
		"Add a log_slow_filter to avoid slow query logging of scrapes. NOTE: Not supported by Oracle MySQL.",
	).Default("false").Bool()
	collectHeartbeatDatabase = kingpin.Flag(
		"collect.heartbeat.database",
		"Database from where to collect heartbeat data",
	).Default(config.DefaultHeartbeatDatabase).String()
	collectHeartbeatTable = kingpin.Flag(
		"collect.heartbeat.table",
		"Table from where to collect heartbeat data",
	).Default(config.DefaultHeartbeatTable).String()
	collectHeartbeatUTC = kingpin.Flag(
		"collect.heartbeat.utc",
		"Use UTC for timestamps of the current server (`pt-heartbeat` is called with `--utc`)",
	).Bool()
	processlistMinTime = kingpin.Flag(
		"collect.info_schema.processlist.min_time",
		"Minimum time a thread must be in each state to be counted",
	).Default(strconv.Itoa(config.DefaultInfoSchemaProcesslistMinTime)).Int()
	processesByUserFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_user",
		"Enable collecting the number of processes by user",
	).Default(strconv.FormatBool(config.DefaultInfoSchemaProcesslistProcessesByUser)).Bool()
	processesByHostFlag = kingpin.Flag(
		"collect.info_schema.processlist.processes_by_host",
		"Enable collecting the number of processes by host",
	).Default(strconv.FormatBool(config.DefaultInfoSchemaProcesslistProcessesByHost)).Bool()
	tableSchemaDatabases = kingpin.Flag(
		"collect.info_schema.tables.databases",
		"The list of databases to collect table stats for, or '*' for all",
	).Default(config.DefaultInfoSchemaTablesDatabases).String()
	perfEventsStatementsLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.limit",
		"Limit the number of events statements digests by response time",
	).Default(strconv.Itoa(config.DefaultPerfSchemaEventsStatementsLimit)).Int()
	perfEventsStatementsTimeLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.timelimit",
		"Limit how old the 'last_seen' events statements can be, in seconds",
	).Default(strconv.Itoa(config.DefaultPerfSchemaEventsStatementsTimeLimit)).Int()
	perfEventsStatementsDigestTextLimit = kingpin.Flag(
		"collect.perf_schema.eventsstatements.digest_text_limit",
		"Maximum length of the normalized statement text",
	).Default(strconv.Itoa(config.DefaultPerfSchemaEventsStatementsDigestTextLimit)).Int()
	perfEventsStatementsExcludeSchemas = kingpin.Flag(
		"collect.perf_schema.eventsstatements.exclude_schemas",
		"Additional schema name to exclude (always excludes mysql, performance_schema, information_schema). Repeatable",
	).Strings()
	performanceSchemaFileInstancesFilter = kingpin.Flag(
		"collect.perf_schema.file_instances.filter",
		"RegEx file_name filter for performance_schema.file_summary_by_instance",
	).Default(config.DefaultPerfSchemaFileInstancesFilter).String()
	performanceSchemaFileInstancesRemovePrefix = kingpin.Flag(
		"collect.perf_schema.file_instances.remove_prefix",
		"Remove path prefix in performance_schema.file_summary_by_instance",
	).Default(config.DefaultPerfSchemaFileInstancesRemovePrefix).String()
	performanceSchemaMemoryEventsRemovePrefix = kingpin.Flag(
		"collect.perf_schema.memory_events.remove_prefix",
		"Remove instrument prefix in performance_schema.memory_summary_global_by_event_name",
	).Default(config.DefaultPerfSchemaMemoryEventsRemovePrefix).String()
	userPrivilegesFlag = kingpin.Flag(
		"collect.mysql.user.privileges",
		"Enable collecting user privileges from mysql.user",
	).Default(strconv.FormatBool(config.DefaultMysqlUserPrivileges)).Bool()
	toolkitFlags = webflag.AddFlags(kingpin.CommandLine, ":9104")
	c            *config.AuthConfigHandler
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

func getScrapeTimeoutSeconds(r *http.Request, offset float64) (float64, error) {
	var timeoutSeconds float64
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error
		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse timeout from Prometheus header: %v", err)
		}
	}
	if timeoutSeconds == 0 {
		return 0, nil
	}
	if timeoutSeconds < 0 {
		return 0, fmt.Errorf("timeout value from Prometheus header is invalid: %f", timeoutSeconds)
	}

	if offset >= timeoutSeconds {
		// Ignore timeout offset if it doesn't leave time to scrape.
		return 0, fmt.Errorf("timeout offset (%f) should be lower than prometheus scrape timeout (%f)", offset, timeoutSeconds)
	} else {
		// Subtract timeout offset from timeout.
		timeoutSeconds -= offset
	}
	return timeoutSeconds, nil
}

func configForCollectParams(cfg config.Config, collectParams []string) config.Config {
	enabledScrapers := collector.EnabledScrapers(cfg)
	filteredScrapers := filterScrapers(enabledScrapers, collectParams)
	if len(collectParams) == 0 || len(filteredScrapers) == len(enabledScrapers) {
		return cfg
	}

	cfg.Collectors = make(map[string]bool, len(cfg.Collectors))
	for name := range config.DefaultCollectorConfig() {
		cfg.Collectors[name] = false
	}
	for _, scraper := range filteredScrapers {
		cfg.Collectors[scraper.Name()] = true
	}
	return cfg
}

func configFromFlags(collectorFlags map[string]*bool) config.Config {
	cfg := config.NewConfigWithDefaults()
	for name, enabled := range collectorFlags {
		cfg.Collectors[name] = *enabled
	}
	cfg.TimeoutOffset = *timeoutOffset
	cfg.EnableExporterLockWaitTimeout = *enableExporterLockTimeout
	cfg.ExporterLockWaitTimeout = *exporterLockTimeout
	cfg.SlowLogFilter = *slowLogFilter
	cfg.Heartbeat = config.HeartbeatConfig{
		Database: *collectHeartbeatDatabase,
		Table:    *collectHeartbeatTable,
		UTC:      *collectHeartbeatUTC,
	}
	cfg.InfoSchemaProcesslist = config.InfoSchemaProcesslistConfig{
		MinTime:         *processlistMinTime,
		ProcessesByUser: *processesByUserFlag,
		ProcessesByHost: *processesByHostFlag,
	}
	cfg.InfoSchemaTables = config.InfoSchemaTablesConfig{
		Databases: *tableSchemaDatabases,
	}
	cfg.PerfSchemaEventsStatements = config.PerfSchemaEventsStatementsConfig{
		Limit:           *perfEventsStatementsLimit,
		TimeLimit:       *perfEventsStatementsTimeLimit,
		DigestTextLimit: *perfEventsStatementsDigestTextLimit,
		ExcludeSchemas:  *perfEventsStatementsExcludeSchemas,
	}
	cfg.PerfSchemaFileInstances = config.PerfSchemaFileInstancesConfig{
		Filter:       *performanceSchemaFileInstancesFilter,
		RemovePrefix: *performanceSchemaFileInstancesRemovePrefix,
	}
	cfg.PerfSchemaMemoryEvents = config.PerfSchemaMemoryEventsConfig{
		RemovePrefix: *performanceSchemaMemoryEventsRemovePrefix,
	}
	cfg.MysqlUser = config.MysqlUserConfig{
		Privileges: *userPrivilegesFlag,
	}
	return cfg
}

func init() {
	prometheus.MustRegister(versioncollector.NewCollector("mysqld_exporter"))
}

func newHandler(baseConfig config.Config, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dsn string
		var err error
		target := ""
		q := r.URL.Query()
		if q.Has("target") {
			target = q.Get("target")
		}

		authConfig := c.GetConfig()
		cfgsection, ok := authConfig.Sections["client"]
		if !ok {
			logger.Error("Failed to parse section [client] from config file", "err", err)
		}
		if dsn, err = cfgsection.FormDSN(target); err != nil {
			logger.Error("Failed to form dsn from section [client]", "err", err)
		}

		collect := q["collect[]"]

		// Use request context for cancellation when connection gets closed.
		ctx := r.Context()
		// If a timeout is configured via the Prometheus header, add it to the context.
		timeoutSeconds, err := getScrapeTimeoutSeconds(r, baseConfig.TimeoutOffset)
		if err != nil {
			logger.Error("Error getting timeout from Prometheus header", "err", err)
		}
		if timeoutSeconds > 0 {
			// Create new timeout context with request context as parent.
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds*float64(time.Second)))
			defer cancel()
			// Overwrite request with timeout context.
			r = r.WithContext(ctx)
		}

		cfg := configForCollectParams(baseConfig, collect)
		cfg.DataSourceName = dsn
		if err := cfg.Validate(); err != nil {
			logger.Error("Invalid runtime config", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		registry := prometheus.NewRegistry()
		runtime, err := collector.NewRuntimeWithContext(ctx, &cfg, logger)
		if err != nil {
			logger.Error("Error creating runtime", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, c := range runtime.Collectors() {
			registry.MustRegister(c)
		}

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
	defaultConfig := config.NewConfigWithDefaults()
	scraperFlags := map[string]*bool{}
	for _, scraper := range collector.AllScrapers(defaultConfig) {
		enabledByDefault := defaultConfig.Collectors[scraper.Name()]
		defaultOn := "false"
		if enabledByDefault {
			defaultOn = "true"
		}

		f := kingpin.Flag(
			"collect."+scraper.Name(),
			scraper.Help(),
		).Default(defaultOn).Bool()

		scraperFlags[scraper.Name()] = f
	}

	// Parse flags.
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting mysqld_exporter", "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())

	var err error
	c, err = config.NewAuthConfigHandler(prometheus.DefaultRegisterer)
	if err != nil {
		logger.Error("Error creating config handler", "err", err)
		os.Exit(1)
	}
	if err = c.ReloadConfig(*configMycnf, *mysqldAddress, *mysqldUser, *tlsInsecureSkipVerify, logger); err != nil {
		logger.Info("Error parsing host config", "file", *configMycnf, "err", err)
		os.Exit(1)
	}

	// Register only scrapers enabled by flag.
	cfg := configFromFlags(scraperFlags)
	for _, scraper := range collector.EnabledScrapers(cfg) {
		logger.Info("Scraper enabled", "scraper", scraper.Name())
	}
	for scraperName, enabled := range scraperFlags {
		if *enabled {
			cfg.Collectors[scraperName] = true
		}
	}
	handlerFunc := newHandler(cfg, logger)
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
			logger.Error("Error creating landing page", "err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}
	http.HandleFunc("/probe", handleProbe(cfg, logger))
	http.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		if err = c.ReloadConfig(*configMycnf, *mysqldAddress, *mysqldUser, *tlsInsecureSkipVerify, logger); err != nil {
			logger.Warn("Error reloading host config", "file", *configMycnf, "error", err)
			return
		}
		_, _ = w.Write([]byte(`ok`))
	})
	srv := &http.Server{}
	if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
