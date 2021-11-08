// Copyright 2020 The Prometheus Authors
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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestScrapePerfMemoryEvents(t *testing.T) {
	_, err := kingpin.CommandLine.Parse([]string{})
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"EVENT_NAME",
		"SUM_NUMBER_OF_BYTES_ALLOC",
		"SUM_NUMBER_OF_BYTES_FREE",
		"CURRENT_NUMBER_OF_BYTES_USED",
	}

	rows := sqlmock.NewRows(columns).
		AddRow("memory/innodb/event1", "1001", "500", "501").
		AddRow("memory/performance_schema/event1", "6000", "7", "-83904").
		AddRow("memory/innodb/event2", "2002", "1000", "1002").
		AddRow("memory/sql/event1", "30", "4", "26")
	mock.ExpectQuery(sanitizeQuery(perfMemoryEventsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfMemoryEvents{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			panic(fmt.Sprintf("error calling function on test: %s", err))
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"event_name": "innodb/event1"}, value: 1001, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "innodb/event1"}, value: 500, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "innodb/event1"}, value: 501, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"event_name": "performance_schema/event1"}, value: 6000, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "performance_schema/event1"}, value: 7, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "performance_schema/event1"}, value: -83904, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"event_name": "innodb/event2"}, value: 2002, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "innodb/event2"}, value: 1000, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "innodb/event2"}, value: 1002, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"event_name": "sql/event1"}, value: 30, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "sql/event1"}, value: 4, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"event_name": "sql/event1"}, value: 26, metricType: dto.MetricType_GAUGE},
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
