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

// Scrape `SHOW ENGINE TOKUDB STATUS`.

package collector

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Subsystem.
	tokudb = "engine_tokudb"
	// Query.
	engineTokudbStatusQuery = `SHOW ENGINE TOKUDB STATUS`
)

// ScrapeEngineTokudbStatus scrapes from `SHOW ENGINE TOKUDB STATUS`.
type ScrapeEngineTokudbStatus struct{}

// Name of the Scraper. Should be unique.
func (ScrapeEngineTokudbStatus) Name() string {
	return "engine_tokudb_status"
}

// Help describes the role of the Scraper.
func (ScrapeEngineTokudbStatus) Help() string {
	return "Collect from SHOW ENGINE TOKUDB STATUS"
}

// Version of MySQL from which scraper is available.
func (ScrapeEngineTokudbStatus) Version() float64 {
	return 5.6
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeEngineTokudbStatus) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	tokudbRows, err := db.QueryContext(ctx, engineTokudbStatusQuery)
	if err != nil {
		return err
	}
	defer tokudbRows.Close()

	var temp, key string
	var val sql.RawBytes

	for tokudbRows.Next() {
		if err := tokudbRows.Scan(&temp, &key, &val); err != nil {
			return err
		}
		key = strings.ToLower(key)
		if floatVal, ok := parseStatus(val); ok {
			ch <- prometheus.MustNewConstMetric(
				newDesc(tokudb, sanitizeTokudbMetric(key), "Generic metric from SHOW ENGINE TOKUDB STATUS."),
				prometheus.UntypedValue,
				floatVal,
			)
		}
	}
	return nil
}

func sanitizeTokudbMetric(metricName string) string {
	replacements := map[string]string{
		">": "",
		",": "",
		":": "",
		"(": "",
		")": "",
		" ": "_",
		"-": "_",
		"+": "and",
		"/": "and",
	}
	for r := range replacements {
		metricName = strings.Replace(metricName, r, replacements[r], -1)
	}
	return metricName
}

// check interface
var _ Scraper = ScrapeEngineTokudbStatus{}
