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
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeInnodbCmpMem(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"page_size", "buffer_pool", "pages_used", "pages_free", "relocation_ops", "relocation_time"}
	rows := sqlmock.NewRows(columns).
		AddRow("1024", "0", 30, 40, 50, 6000)
	mock.ExpectQuery(sanitizeQuery(innodbCmpMemQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeInnodbCmpMem{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"page_size": "1024", "buffer_pool": "0"}, value: 30, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer_pool": "0"}, value: 40, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer_pool": "0"}, value: 50, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"page_size": "1024", "buffer_pool": "0"}, value: 6, metricType: dto.MetricType_COUNTER},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
