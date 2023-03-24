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

package collector

import (
	"context"
	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"regexp"
	"strconv"
	"testing"
)

func TestScrapeSysUserSummary(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"user",
		"statemets",
		"statement_latency",
		"table_scans",
		"file_ios",
		"file_io_latency",
		"current_connections",
		"total_connections",
		"unique_hosts",
		"current_memory",
		"total_memory_allocated",
	}
	rows := sqlmock.NewRows(columns)
	queryResults := [][]driver.Value{
		{
			"user1",
			"110",
			"120",
			"140",
			"150",
			"160",
			"170",
			"180",
			"190",
			"110",
			"111",
		},
		{
			"user2",
			"210",
			"220",
			"240",
			"250",
			"260",
			"270",
			"280",
			"290",
			"210",
			"211",
		},
	}
	expectedMetrics := []MetricResult{}
	// Register the query results with mock SQL driver and assemble expected metric results list
	for _, row := range queryResults {
		rows.AddRow(row...)
		user := row[0]
		for i, metricsValue := range row {
			if i == 0 {
				continue
			}
			metricType := dto.MetricType_COUNTER
			// Current Connections and Current Memory are gauges
			if i == 6 || i == 9 {
				metricType = dto.MetricType_GAUGE
			}
			value, err := strconv.ParseFloat(metricsValue.(string), 64)
			if err != nil {
				t.Errorf("Failed to parse result value as float64: %+v", err)
			}
			// Statement latency & IO latency are latencies in picoseconds, convert them to seconds
			if i == 2 || i == 5 {
				value = value / picoSeconds
			}
			expectedMetrics = append(expectedMetrics, MetricResult{
				labels:     labelMap{"user": user.(string)},
				value:      value,
				metricType: metricType,
			})
		}
	}

	mock.ExpectQuery(sanitizeQuery(regexp.QuoteMeta(sysUserSummaryQuery))).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)

	go func() {
		if err = (ScrapeSysUserSummary{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	// Ensure metrics look OK
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expectedMetrics {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
