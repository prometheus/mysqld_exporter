package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/ini.v1"

	"github.com/prometheus/mysqld_exporter/collector"
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
	slowLogFilter = flag.Bool(
		"log_slow_filter", false,
		"Add a log_slow_filter to avoid exessive MySQL slow logging.  NOTE: Not supported by Oracle MySQL.",
	)
	collectProcesslist = flag.Bool(
		"collect.info_schema.processlist", false,
		"Collect current thread state counts from the information_schema.processlist",
	)
	collectTableSchema = flag.Bool(
		"collect.info_schema.tables", true,
		"Collect metrics from information_schema.tables",
	)
	collectInnodbTablespaces = flag.Bool(
		"collect.info_schema.innodb_tablespaces", false,
		"Collect metrics from information_schema.innodb_sys_tablespaces",
	)
	collectInnodbMetrics = flag.Bool(
		"collect.info_schema.innodb_metrics", false,
		"Collect metrics from information_schema.innodb_metrics",
	)
	collectGlobalStatus = flag.Bool(
		"collect.global_status", true,
		"Collect from SHOW GLOBAL STATUS",
	)
	collectGlobalVariables = flag.Bool(
		"collect.global_variables", true,
		"Collect from SHOW GLOBAL VARIABLES",
	)
	collectSlaveStatus = flag.Bool(
		"collect.slave_status", true,
		"Collect from SHOW SLAVE STATUS",
	)
	collectAutoIncrementColumns = flag.Bool(
		"collect.auto_increment.columns", false,
		"Collect auto_increment columns and max values from information_schema",
	)
	collectBinlogSize = flag.Bool(
		"collect.binlog_size", false,
		"Collect the current size of all registered binlog files",
	)
	collectPerfTableIOWaits = flag.Bool(
		"collect.perf_schema.tableiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_table",
	)
	collectPerfIndexIOWaits = flag.Bool(
		"collect.perf_schema.indexiowaits", false,
		"Collect metrics from performance_schema.table_io_waits_summary_by_index_usage",
	)
	collectPerfTableLockWaits = flag.Bool(
		"collect.perf_schema.tablelocks", false,
		"Collect metrics from performance_schema.table_lock_waits_summary_by_table",
	)
	collectPerfEventsStatements = flag.Bool(
		"collect.perf_schema.eventsstatements", false,
		"Collect metrics from performance_schema.events_statements_summary_by_digest",
	)
	collectPerfEventsWaits = flag.Bool(
		"collect.perf_schema.eventswaits", false,
		"Collect metrics from performance_schema.events_waits_summary_global_by_event_name",
	)
	collectPerfFileEvents = flag.Bool(
		"collect.perf_schema.file_events", false,
		"Collect metrics from performance_schema.file_summary_by_event_name",
	)
	collectPerfFileInstances = flag.Bool(
		"collect.perf_schema.file_instances", false,
		"Collect metrics from performance_schema.file_summary_by_instance",
	)
	collectUserStat = flag.Bool("collect.info_schema.userstats", false,
		"If running with userstat=1, set to true to collect user statistics",
	)
	collectClientStat = flag.Bool("collect.info_schema.clientstats", false,
		"If running with userstat=1, set to true to collect client statistics",
	)
	collectTableStat = flag.Bool("collect.info_schema.tablestats", false,
		"If running with userstat=1, set to true to collect table statistics",
	)
	collectQueryResponseTime = flag.Bool("collect.info_schema.query_response_time", false,
		"Collect query response time distribution if query_response_time_stats is ON.",
	)
	collectEngineTokudbStatus = flag.Bool("collect.engine_tokudb_status", false,
		"Collect from SHOW ENGINE TOKUDB STATUS",
	)
	collectEngineInnodbStatus = flag.Bool("collect.engine_innodb_status", false,
		"Collect from SHOW ENGINE INNODB STATUS",
	)
	collectHeartbeat = flag.Bool(
		"collect.heartbeat", false,
		"Collect from heartbeat",
	)
	collectHeartbeatDatabase = flag.String(
		"collect.heartbeat.database", "heartbeat",
		"Database from where to collect heartbeat data",
	)
	collectHeartbeatTable = flag.String(
		"collect.heartbeat.table", "heartbeat",
		"Table from where to collect heartbeat data",
	)
)

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

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("mysqld_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting mysqld_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if len(dsn) == 0 {
		var err error
		if dsn, err = parseMycnf(*configMycnf); err != nil {
			log.Fatal(err)
		}
	}

	collect := collector.Collect{
		SlowLogFilter:        *slowLogFilter,
		Processlist:          *collectProcesslist,
		TableSchema:          *collectTableSchema,
		InnodbTablespaces:    *collectInnodbTablespaces,
		InnodbMetrics:        *collectInnodbMetrics,
		GlobalStatus:         *collectGlobalStatus,
		GlobalVariables:      *collectGlobalVariables,
		SlaveStatus:          *collectSlaveStatus,
		AutoIncrementColumns: *collectAutoIncrementColumns,
		BinlogSize:           *collectBinlogSize,
		PerfTableIOWaits:     *collectPerfTableIOWaits,
		PerfIndexIOWaits:     *collectPerfIndexIOWaits,
		PerfTableLockWaits:   *collectPerfTableLockWaits,
		PerfEventsStatements: *collectPerfEventsStatements,
		PerfEventsWaits:      *collectPerfEventsWaits,
		PerfFileEvents:       *collectPerfFileEvents,
		PerfFileInstances:    *collectPerfFileInstances,
		UserStat:             *collectUserStat,
		ClientStat:           *collectClientStat,
		TableStat:            *collectTableStat,
		QueryResponseTime:    *collectQueryResponseTime,
		EngineTokudbStatus:   *collectEngineTokudbStatus,
		EngineInnodbStatus:   *collectEngineInnodbStatus,
		Heartbeat:            *collectHeartbeat,
		HeartbeatDatabase:    *collectHeartbeatDatabase,
		HeartbeatTable:       *collectHeartbeatTable,
	}

	c := collector.New(dsn, collect)
	prometheus.MustRegister(c)

	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
