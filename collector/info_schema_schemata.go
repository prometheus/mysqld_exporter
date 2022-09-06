// Copyright 2022 The Prometheus Authors
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

// Scrape `information_schema.tables`.

package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric descriptors.
var (
	infoSchemaAmount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, informationSchema, "count"),
		"The count of user schemas on server",
		nil, nil,
	)
)

const (
	schemaCountQuery = `
	SELECT COUNT(*) FROM information_schema.schemata 
	WHERE schema_name NOT IN ('mysql','information_schema');
	`
)

// ScrapeSchemaAmount collects from `information_schema.tables`.
type ScrapeSchemaAmount struct{}

// Name of the Scraper. Should be unique.
func (ScrapeSchemaAmount) Name() string {
	return informationSchema + ".schemata"
}

// Help describes the role of the Scraper.
func (ScrapeSchemaAmount) Help() string {
	return "Collect metrics from information_schema.schemata"
}

// Version of MySQL from which scraper is available.
func (ScrapeSchemaAmount) Version() float64 {
	return 5.1
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeSchemaAmount) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	dbCount, err := db.QueryContext(ctx, schemaCountQuery)
	if err != nil {
		return err
	}
	defer dbCount.Close()

	dbAmount := 0
	dbCount.Next()
	dbCount.Scan(&dbAmount)

	ch <- prometheus.MustNewConstMetric(
		infoSchemaAmount, prometheus.GaugeValue,
		float64(dbAmount),
	)

	return nil
}

// check interface
var _ Scraper = ScrapeSchemaAmount{}
