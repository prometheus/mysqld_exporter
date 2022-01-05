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

func TestScrapeSlaveStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Master_Host", "Read_Master_Log_Pos", "Slave_IO_Running", "Slave_SQL_Running", "Seconds_Behind_Master"}
	rows := sqlmock.NewRows(columns).
		AddRow("127.0.0.1", "1", "Connecting", "Yes", "2")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE STATUS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveStatus{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 0, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 2, metricType: dto.MetricType_UNTYPED},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
