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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestScrapePerfFileInstances(t *testing.T) {
	_, err := kingpin.CommandLine.Parse([]string{"--collect.perf_schema.file_instances.filter", ""})
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"FILE_NAME", "EVENT_NAME", "COUNT_READ", "COUNT_WRITE", "SUM_NUMBER_OF_BYTES_READ", "SUM_NUMBER_OF_BYTES_WRITE"}

	rows := sqlmock.NewRows(columns).
		AddRow("/var/lib/mysql/db1/file", "event1", "3", "4", "725", "128").
		AddRow("/var/lib/mysql/db2/file", "event2", "23", "12", "3123", "967").
		AddRow("db3/file", "event3", "45", "32", "1337", "326")
	mock.ExpectQuery(sanitizeQuery(perfFileInstancesQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfFileInstances{}).Scrape(context.Background(), db, ch); err != nil {
			panic(fmt.Sprintf("error calling function on test: %s", err))
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"file_name": "db1/file", "event_name": "event1", "mode": "read"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db1/file", "event_name": "event1", "mode": "write"}, value: 4, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db1/file", "event_name": "event1", "mode": "read"}, value: 725, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db1/file", "event_name": "event1", "mode": "write"}, value: 128, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db2/file", "event_name": "event2", "mode": "read"}, value: 23, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db2/file", "event_name": "event2", "mode": "write"}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db2/file", "event_name": "event2", "mode": "read"}, value: 3123, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db2/file", "event_name": "event2", "mode": "write"}, value: 967, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db3/file", "event_name": "event3", "mode": "read"}, value: 45, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db3/file", "event_name": "event3", "mode": "write"}, value: 32, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db3/file", "event_name": "event3", "mode": "read"}, value: 1337, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"file_name": "db3/file", "event_name": "event3", "mode": "write"}, value: 326, metricType: dto.MetricType_COUNTER},
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
