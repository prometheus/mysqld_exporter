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

package collector

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/log"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestScrapeInnodbMetrics(t *testing.T) {
	// Suppress a log messages
	log.AddFlags(kingpin.CommandLine)
	_, err := kingpin.CommandLine.Parse([]string{"--log.level", "fatal"})
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"name", "subsystem", "type", "comment", "count"}
	rows := sqlmock.NewRows(columns).
		AddRow("lock_timeouts", "lock", "counter", "Number of lock timeouts", 0).
		AddRow("buffer_pool_reads", "buffer", "status_counter", "Number of reads directly from disk (innodb_buffer_pool_reads)", 1).
		AddRow("buffer_pool_size", "server", "value", "Server buffer pool size (all buffer pools) in bytes", 2).
		AddRow("buffer_page_read_system_page", "buffer_page_io", "counter", "Number of System Pages read", 3).
		AddRow("buffer_page_written_undo_log", "buffer_page_io", "counter", "Number of Undo Log Pages written", 4).
		AddRow("buffer_pool_pages_dirty", "buffer", "gauge", "Number of dirt buffer pool pages", 5).
		AddRow("buffer_pool_pages_data", "buffer", "gauge", "Number of data buffer pool pages", 6).
		AddRow("buffer_pool_pages_total", "buffer", "gauge", "Number of total buffer pool pages", 7).
		AddRow("NOPE", "buffer_page_io", "counter", "An invalid buffer_page_io metric", 999)
	mock.ExpectQuery(sanitizeQuery(infoSchemaInnodbMetricsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeInnodbMetrics{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"type": "system_page"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"type": "undo_log"}, value: 4, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 5, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "data"}, value: 6, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
