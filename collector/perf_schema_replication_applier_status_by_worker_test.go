// Copyright 2019 The Prometheus Authors
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
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapePerfReplicationApplierStatsByWorker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"CHANNEL_NAME",
		"WORKER_ID",
		"LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP",
		"LAST_APPLIED_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP",
		"LAST_APPLIED_TRANSACTION_START_APPLY_TIMESTAMP",
		"LAST_APPLIED_TRANSACTION_END_APPLY_TIMESTAMP",
		"APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP",
		"APPLYING_TRANSACTION_IMMEDIATE_COMMIT_TIMESTAMP",
		"APPLYING_TRANSACTION_START_APPLY_TIMESTAMP",
	}

	timeZero := "0000-00-00 00:00:00.000000"

	stubTime := time.Date(2019, 3, 14, 0, 0, 0, int(time.Millisecond), time.UTC)
	rows := sqlmock.NewRows(columns).
		AddRow("dummy_0", "0", timeZero, timeZero, timeZero, timeZero, timeZero, timeZero, timeZero).
		AddRow("dummy_1", "1", stubTime.Format(timeLayout), stubTime.Add(1*time.Minute).Format(timeLayout), stubTime.Add(2*time.Minute).Format(timeLayout), stubTime.Add(3*time.Minute).Format(timeLayout), stubTime.Add(4*time.Minute).Format(timeLayout), stubTime.Add(5*time.Minute).Format(timeLayout), stubTime.Add(6*time.Minute).Format(timeLayout))
	mock.ExpectQuery(sanitizeQuery(perfReplicationApplierStatsByWorkerQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfReplicationApplierStatsByWorker{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_0", "member_id": "0"}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521600001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521660001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521720001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521780001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521840001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521900001e+9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "dummy_1", "member_id": "1"}, value: 1.552521960001e+9, metricType: dto.MetricType_GAUGE},
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
