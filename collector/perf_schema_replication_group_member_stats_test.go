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

func TestScrapePerfReplicationGroupMemberStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"CHANNEL_NAME",
		"VIEW_ID",
		"MEMBER_ID",
		"COUNT_TRANSACTIONS_IN_QUEUE",
		"COUNT_TRANSACTIONS_CHECKED",
		"COUNT_CONFLICTS_DETECTED",
		"COUNT_TRANSACTIONS_ROWS_VALIDATING",
		"TRANSACTIONS_COMMITTED_ALL_MEMBERS",
		"LAST_CONFLICT_FREE_TRANSACTION",
		"COUNT_TRANSACTIONS_REMOTE_IN_APPLIER_QUEUE",
		"COUNT_TRANSACTIONS_REMOTE_APPLIED",
		"COUNT_TRANSACTIONS_LOCAL_PROPOSED",
		"COUNT_TRANSACTIONS_LOCAL_ROLLBACK",
	}
	rows := sqlmock.NewRows(columns).
		AddRow(
			"group_replication_applier",
			"15813535259046852:43",
			"e14c4f71-025f-11ea-b800-0620049edbec",
			float64(0),
			float64(7389775),
			float64(1),
			float64(48),
			"0515b3c2-f59f-11e9-881b-0620049edbec:1-15270987,\n8f782839-34f7-11e7-a774-060ac4f023ae:4-39:2387-161606",
			"0515b3c2-f59f-11e9-881b-0620049edbec:15271011",
			float64(2),
			float64(22),
			float64(7389759),
			float64(7),
		)
	mock.ExpectQuery(sanitizeQuery(perfReplicationGroupMemberStatsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfReplicationGroupMemberStats{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: float64(7389775), metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: float64(1), metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: float64(48), metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: float64(22), metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: float64(7389759), metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: float64(7), metricType: dto.MetricType_COUNTER},
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
