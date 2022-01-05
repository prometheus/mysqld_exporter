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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeReplicaHost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"SERVER_ID", "ROLE", "CPU", "MASTER_SLAVE_LATENCY_IN_MICROSECONDS", "REPLICA_LAG_IN_MILLISECONDS", "LOG_STREAM_SPEED_IN_KiB_PER_SECOND", "CURRENT_REPLAY_LATENCY_IN_MICROSECONDS"}
	rows := sqlmock.NewRows(columns).
		AddRow("dbtools-cluster-us-west-2c", "reader", 1.2531328201293945, 250000, 20.069000244140625, 2.0368164549078225, 500000).
		AddRow("dbtools-cluster-writer", "writer", 1.9607843160629272, 250000, 0, 2.0368164549078225, 0)
	mock.ExpectQuery(sanitizeQuery(replicaHostQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeReplicaHost{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"server_id": "dbtools-cluster-us-west-2c", "role": "reader"}, value: 1.2531328201293945, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-us-west-2c", "role": "reader"}, value: 0.25, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-us-west-2c", "role": "reader"}, value: 0.020069000244140625, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-us-west-2c", "role": "reader"}, value: 2.0368164549078225, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-us-west-2c", "role": "reader"}, value: 0.5, metricType: dto.MetricType_GAUGE},

		{labels: labelMap{"server_id": "dbtools-cluster-writer", "role": "writer"}, value: 1.9607843160629272, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-writer", "role": "writer"}, value: 0.25, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-writer", "role": "writer"}, value: 0.0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-writer", "role": "writer"}, value: 2.0368164549078225, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "dbtools-cluster-writer", "role": "writer"}, value: 0.0, metricType: dto.MetricType_GAUGE},
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
