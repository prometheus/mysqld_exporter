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

// Scrape `information_schema.query_response_time*` tables.

package collector

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const queryResponseCheckQuery = `SELECT @@query_response_time_stats`

var (
	// Use uppercase for table names, otherwise read/write split will return the same results as total
	// due to the bug.
	queryResponseTimeQueries = [3]string{
		"SELECT TIME, COUNT, TOTAL FROM INFORMATION_SCHEMA.QUERY_RESPONSE_TIME",
		"SELECT TIME, COUNT, TOTAL FROM INFORMATION_SCHEMA.QUERY_RESPONSE_TIME_READ",
		"SELECT TIME, COUNT, TOTAL FROM INFORMATION_SCHEMA.QUERY_RESPONSE_TIME_WRITE",
	}
)

func processQueryResponseTimeTable(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, query string, desc *prometheus.Desc) error {
	queryDistributionRows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer queryDistributionRows.Close()

	var (
		length       string
		count        uint64
		total        string
		histogramCnt uint64
		histogramSum float64
		countBuckets = map[float64]uint64{}
	)

	for queryDistributionRows.Next() {
		err = queryDistributionRows.Scan(
			&length,
			&count,
			&total,
		)
		if err != nil {
			return err
		}

		length, _ := strconv.ParseFloat(strings.TrimSpace(length), 64)
		total, _ := strconv.ParseFloat(strings.TrimSpace(total), 64)
		histogramCnt += count
		histogramSum += total
		// Special case for "TOO LONG" row where we take into account the count field which is the only available
		// and do not add it as a part of histogram or metric
		if length == 0 {
			continue
		}
		countBuckets[length] = histogramCnt
	}
	// Create histogram with query counts
	ch <- prometheus.MustNewConstHistogram(desc, histogramCnt, histogramSum, countBuckets)
	return nil
}

// ScrapeQueryResponseTime collects from `information_schema.query_response_time`.
type ScrapeQueryResponseTime struct{}

// Name of the Scraper. Should be unique.
func (ScrapeQueryResponseTime) Name() string {
	return "info_schema.query_response_time"
}

// Help describes the role of the Scraper.
func (ScrapeQueryResponseTime) Help() string {
	return "Collect query response time distribution if query_response_time_stats is ON."
}

// Version of MySQL from which scraper is available.
func (ScrapeQueryResponseTime) Version() float64 {
	return 5.5
}

// Scrape collects data from database connection and sends it over channel as prometheus metric.
func (ScrapeQueryResponseTime) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, constLabels prometheus.Labels) error {
	var queryStats uint8
	err := db.QueryRowContext(ctx, queryResponseCheckQuery).Scan(&queryStats)
	if err != nil {
		log.Debugln("Query response time distribution is not present.")
		return nil
	}
	if queryStats == 0 {
		log.Debugln("query_response_time_stats is OFF.")
		return nil
	}

	descs := [3]*prometheus.Desc{
		newDesc(informationSchema, "query_response_time_seconds", "The number of all queries by duration they took to execute.", constLabels),
		newDesc(informationSchema, "read_query_response_time_seconds", "The number of read queries by duration they took to execute.", constLabels),
		newDesc(informationSchema, "write_query_response_time_seconds", "The number of write queries by duration they took to execute.", constLabels),
	}
	for i, query := range queryResponseTimeQueries {
		err := processQueryResponseTimeTable(ctx, db, ch, query, descs[i])
		// The first query should not fail if query_response_time_stats is ON,
		// unlike the other two when the read/write tables exist only with Percona Server 5.6/5.7.
		if i == 0 && err != nil {
			return err
		}
	}
	return nil
}

// check interface
var _ Scraper = ScrapeQueryResponseTime{}
