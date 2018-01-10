package collector

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

var _ Scraper = &DefaultScraper{}

type Scraper interface {
	Scrape(db *sql.DB, ch chan<- prometheus.Metric) error
	Name() string
	Version() float64
}

type DefaultScraper struct {
	name    string
	f       func(db *sql.DB, ch chan<- prometheus.Metric) error
	version float64
}

func (c *DefaultScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric) error {
	return c.f(db, ch)
}

func (c *DefaultScraper) Name() string {
	return c.name
}

func (c *DefaultScraper) Version() float64 {
	return c.version
}

var (
	ScraperGlobalStatus = &DefaultScraper{
		"global_status",
		ScrapeGlobalStatus,
		0,
	}
	ScraperGlobalVariables = &DefaultScraper{
		"global_variables",
		ScrapeGlobalVariables,
		0,
	}
	ScraperSlaveStatus = &DefaultScraper{
		"slave_status",
		ScrapeSlaveStatus,
		0,
	}
	ScraperProcessList = &DefaultScraper{
		"info_schema.processlist",
		ScrapeProcesslist,
		0,
	}
	ScraperTableSchema = &DefaultScraper{
		"info_schema.tables",
		ScrapeTableSchema,
		0,
	}
	ScraperInfoSchemaInnodbTablespaces = &DefaultScraper{
		"info_schema.innodb_sys_tablespaces",
		ScrapeInfoSchemaInnodbTablespaces,
		5.7,
	}
	ScraperInnodbMetrics = &DefaultScraper{
		"info_schema.innodb_metrics",
		ScrapeInnodbMetrics,
		5.6,
	}
	ScraperAutoIncrementColumns = &DefaultScraper{
		"auto_increment.columns",
		ScrapeAutoIncrementColumns,
		0,
	}
	ScraperBinlogSize = &DefaultScraper{
		"binlog_size",
		ScrapeBinlogSize,
		0,
	}
	ScraperPerfTableIOWaits = &DefaultScraper{
		"perf_schema.tableiowaits",
		ScrapePerfTableIOWaits,
		5.6,
	}
	ScraperPerfIndexIOWaits = &DefaultScraper{
		"perf_schema.indexiowaits",
		ScrapePerfIndexIOWaits,
		5.6,
	}
	ScraperPerfTableLockWaits = &DefaultScraper{
		"perf_schema.tablelocks:",
		ScrapePerfTableLockWaits,
		5.6,
	}
	ScraperPerfEventsStatements = &DefaultScraper{
		"perf_schema.eventsstatements:",
		ScrapePerfEventsStatements,
		5.6,
	}
	ScraperPerfEventsWaits = &DefaultScraper{
		"perf_schema.eventswaits",
		ScrapePerfEventsWaits,
		5.5,
	}
	ScraperPerfFileEvents = &DefaultScraper{
		"perf_schema.file_events",
		ScrapePerfFileEvents,
		5.6,
	}
	ScraperPerfFileInstances = &DefaultScraper{
		"perf_schema.file_instances",
		ScrapePerfFileInstances,
		5.5,
	}
	ScraperUserStat = &DefaultScraper{
		"info_schema.userstats",
		ScrapeUserStat,
		0,
	}
	ScraperClientStat = &DefaultScraper{
		"info_schema.clientstats",
		ScrapeClientStat,
		5.5,
	}
	ScraperTableStat = &DefaultScraper{
		"info_schema.tablestats",
		ScrapeTableStat,
		0,
	}
	ScraperQueryResponseTime = &DefaultScraper{
		"info_schema.query_response_time",
		ScrapeQueryResponseTime,
		5.5,
	}
	ScraperEngineTokudbStatus = &DefaultScraper{
		"engine_tokudb_status",
		ScrapeEngineTokudbStatus,
		0,
	}
	ScraperEngineInnodbStatus = &DefaultScraper{
		"engine_innodb_status",
		ScrapeEngineInnodbStatus,
		0,
	}
)

func ScraperHeartbeat(database, table string) *DefaultScraper {
	return &DefaultScraper{
		"heartbeat",
		func(db *sql.DB, ch chan<- prometheus.Metric) error {
			return ScrapeHeartbeat(db, ch, database, table)
		},
		5.1,
	}
}
