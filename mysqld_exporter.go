package main

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/prometheus/client_golang/prometheus"
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
	configMycnf = kingpin.Flag(
		"config.my-cnf",
		"Path to .my.cnf file to read MySQL credentials from.",
	).Default(path.Join(os.Getenv("HOME"), ".my.cnf")).String()
	slowLogFilter = kingpin.Flag(
		"log_slow_filter",
		"Add a log_slow_filter to avoid exessive MySQL slow logging.  NOTE: Not supported by Oracle MySQL.",
	).Default("false").Bool()
	collectProcesslist = kingpin.Flag(
		"collect.info_schema.processlist",
		"Collect current thread state counts from the information_schema.processlist",
	).Default("false").Bool()
	collectTableSchema = kingpin.Flag(
		"collect.info_schema.tables",
		"Collect metrics from information_schema.tables",
	).Default("true").Bool()
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
	).Default("true").Bool()
	collectGlobalVariables = kingpin.Flag(
		"collect.global_variables",
		"Collect from SHOW GLOBAL VARIABLES",
	).Default("true").Bool()
	collectSlaveStatus = kingpin.Flag(
		"collect.slave_status",
		"Collect from SHOW SLAVE STATUS",
	).Default("true").Bool()
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
	collectEngineTokudbStatus = kingpin.Flag(
		"collect.engine_tokudb_status",
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
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("mysqld_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

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
