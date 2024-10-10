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

// Scrape auto_increment column information.

package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const infoSchemaAutoIncrementQuery = `
		SELECT c.table_schema, c.table_name, column_name, auto_increment,
		  pow(2, case data_type
		    when 'tinyint'   then 7
		    when 'smallint'  then 15
		    when 'mediumint' then 23
		    when 'int'       then 31
		    when 'bigint'    then 63
		    end+(column_type like '% unsigned'))-1 as max_int
		  FROM information_schema.columns c
		  STRAIGHT_JOIN information_schema.tables t ON (BINARY c.table_schema=t.table_schema AND BINARY c.table_name=t.table_name)
		  WHERE c.extra = 'auto_increment' AND t.auto_increment IS NOT NULL
		`

// Metric descriptors.
var (
	globalInfoSchemaAutoIncrementDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "auto_increment_column"),
		"The current value of an auto_increment column from information_schema.",
		[]string{"schema", "table", "column"}, nil,
	)
	globalInfoSchemaAutoIncrementMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "auto_increment_column_max"),
		"The max value of an auto_increment column from information_schema.",
		[]string{"schema", "table", "column"}, nil,
	)
)

// ScrapeAutoIncrementColumns collects auto_increment column information.
type ScrapeAutoIncrementColumns struct{}

// Name of the Scraper. Should be unique.
func (ScrapeAutoIncrementColumns) Name() string {
	return "auto_increment.columns"
}

// Help describes the role of the Scraper.
func (ScrapeAutoIncrementColumns) Help() string {
	return "Collect auto_increment columns and max values from information_schema"
}

// Version of MySQL from which scraper is available.
func (ScrapeAutoIncrementColumns) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeAutoIncrementColumns) Scrape(ctx context.Context, instance *instance, ch chan<- prometheus.Metric, logger *slog.Logger) error {
	db := instance.getDB()
	autoIncrementRows, err := db.QueryContext(ctx, infoSchemaAutoIncrementQuery)
	if err != nil {
		return err
	}
	defer autoIncrementRows.Close()

	var (
		schema, table, column string
		value, max            float64
	)

	for autoIncrementRows.Next() {
		if err := autoIncrementRows.Scan(
			&schema, &table, &column, &value, &max,
		); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			globalInfoSchemaAutoIncrementDesc, prometheus.GaugeValue, value,
			schema, table, column,
		)
		ch <- prometheus.MustNewConstMetric(
			globalInfoSchemaAutoIncrementMaxDesc, prometheus.GaugeValue, max,
			schema, table, column,
		)
	}
	return nil
}

// check interface
var _ Scraper = ScrapeAutoIncrementColumns{}
