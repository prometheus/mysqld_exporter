// Scrape `information_schema.tables`.

package collector

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	tableSchemaQuery = `
		SELECT
		    TABLE_SCHEMA,
		    TABLE_NAME,
		    TABLE_TYPE,
		    ifnull(ENGINE, 'NONE') as ENGINE,
		    ifnull(VERSION, '0') as VERSION,
		    ifnull(ROW_FORMAT, 'NONE') as ROW_FORMAT,
		    ifnull(TABLE_ROWS, '0') as TABLE_ROWS,
		    ifnull(DATA_LENGTH, '0') as DATA_LENGTH,
		    ifnull(INDEX_LENGTH, '0') as INDEX_LENGTH,
		    ifnull(DATA_FREE, '0') as DATA_FREE,
		    ifnull(CREATE_OPTIONS, 'NONE') as CREATE_OPTIONS
		  FROM information_schema.tables
		  WHERE TABLE_SCHEMA = '%s'
		`
	dbListQuery = `
		SELECT
		    SCHEMA_NAME
		  FROM information_schema.schemata
		  WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema')
		`
)

var (
	tableSchemaDatabases = flag.String(
		"collect.info_schema.tables.databases", "*",
		"The list of databases to collect table stats for, or '*' for all",
	)
	infoSchemaTablesVersionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_version"),
		"The version number of the table's .frm file",
		[]string{"schema", "table", "type", "engine", "row_format", "create_options"}, nil,
	)
	infoSchemaTablesRowsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_rows"),
		"The estimated number of rows in the table from information_schema.tables",
		[]string{"schema", "table"}, nil,
	)
	infoSchemaTablesSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "table_size"),
		"The size of the table components from information_schema.tables",
		[]string{"schema", "table", "component"}, nil,
	)
)

// ScrapeTableSchema collects from `information_schema.tables`.
type ScrapeTableSchema struct{}

// Name of the Scraper.
func (ScrapeTableSchema) Name() string {
	return informationSchema + ".tables"
}

// Help returns additional information about Scraper.
func (ScrapeTableSchema) Help() string {
	return "Collect metrics from information_schema.tables"
}

// Version of MySQL from which scraper is available.
func (ScrapeTableSchema) Version() float64 {
	return 5.1
}

// Scrape collects data.
func (ScrapeTableSchema) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric) error {
	var dbList []string
	if *tableSchemaDatabases == "*" {
		dbListRows, err := db.QueryContext(ctx, dbListQuery)
		if err != nil {
			return err
		}
		defer dbListRows.Close()

		var database string

		for dbListRows.Next() {
			if err := dbListRows.Scan(
				&database,
			); err != nil {
				return err
			}
			dbList = append(dbList, database)
		}
	} else {
		dbList = strings.Split(*tableSchemaDatabases, ",")
	}

	for _, database := range dbList {
		tableSchemaRows, err := db.QueryContext(ctx, fmt.Sprintf(tableSchemaQuery, database))
		if err != nil {
			return err
		}
		defer tableSchemaRows.Close()

		var (
			tableSchema   string
			tableName     string
			tableType     string
			engine        string
			version       uint64
			rowFormat     string
			tableRows     uint64
			dataLength    uint64
			indexLength   uint64
			dataFree      uint64
			createOptions string
		)

		for tableSchemaRows.Next() {
			err = tableSchemaRows.Scan(
				&tableSchema,
				&tableName,
				&tableType,
				&engine,
				&version,
				&rowFormat,
				&tableRows,
				&dataLength,
				&indexLength,
				&dataFree,
				&createOptions,
			)
			if err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(
				infoSchemaTablesVersionDesc, prometheus.GaugeValue, float64(version),
				tableSchema, tableName, tableType, engine, rowFormat, createOptions,
			)
			ch <- prometheus.MustNewConstMetric(
				infoSchemaTablesRowsDesc, prometheus.GaugeValue, float64(tableRows),
				tableSchema, tableName,
			)
			ch <- prometheus.MustNewConstMetric(
				infoSchemaTablesSizeDesc, prometheus.GaugeValue, float64(dataLength),
				tableSchema, tableName, "data_length",
			)
			ch <- prometheus.MustNewConstMetric(
				infoSchemaTablesSizeDesc, prometheus.GaugeValue, float64(indexLength),
				tableSchema, tableName, "index_length",
			)
			ch <- prometheus.MustNewConstMetric(
				infoSchemaTablesSizeDesc, prometheus.GaugeValue, float64(dataFree),
				tableSchema, tableName, "data_free",
			)
		}
	}

	return nil
}
