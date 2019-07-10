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
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePerfIndexIOWaits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH", "COUNT_INSERT", "COUNT_UPDATE", "COUNT_DELETE", "SUM_TIMER_FETCH", "SUM_TIMER_INSERT", "SUM_TIMER_UPDATE", "SUM_TIMER_DELETE"}
	rows := sqlmock.NewRows(columns).
		// Note, timers are in picoseconds.
		AddRow("database", "table", "index", "10", "11", "12", "13", "14000000000000", "15000000000000", "16000000000000", "17000000000000").
		AddRow("database", "table", "NONE", "20", "21", "22", "23", "24000000000000", "25000000000000", "26000000000000", "27000000000000")
	mock.ExpectQuery(sanitizeQuery(perfIndexIOWaitsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfIndexIOWaits{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "fetch"}, value: 10, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "update"}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "delete"}, value: 13, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "fetch"}, value: 14, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "update"}, value: 16, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "index", "operation": "delete"}, value: 17, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "fetch"}, value: 20, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "insert"}, value: 21, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "update"}, value: 22, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "delete"}, value: 23, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "fetch"}, value: 24, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "insert"}, value: 25, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "update"}, value: 26, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "delete"}, value: 27, metricType: dto.MetricType_COUNTER},
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
