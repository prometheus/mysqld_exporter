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
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestScrapePerfReplicationGroupMembers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"CHANNEL_NAME",
		"MEMBER_ID",
		"MEMBER_HOST",
		"MEMBER_PORT",
		"MEMBER_STATE",
		"MEMBER_ROLE",
		"MEMBER_VERSION",
	}

	rows := sqlmock.NewRows(columns).
		AddRow("group_replication_applier", "uuid1", "hostname1", "3306", "ONLINE", "PRIMARY", "8.0.19").
		AddRow("group_replication_applier", "uuid2", "hostname2", "3306", "ONLINE", "SECONDARY", "8.0.19").
		AddRow("group_replication_applier", "uuid3", "hostname3", "3306", "ONLINE", "SECONDARY", "8.0.19")

	mock.ExpectQuery(sanitizeQuery(perfReplicationGroupMembersQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfReplicationGroupMembers{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid1", "member_host": "hostname1", "member_port": "3306",
			"member_state": "ONLINE", "member_role": "PRIMARY", "member_version": "8.0.19"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid2", "member_host": "hostname2", "member_port": "3306",
			"member_state": "ONLINE", "member_role": "SECONDARY", "member_version": "8.0.19"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid3", "member_host": "hostname3", "member_port": "3306",
			"member_state": "ONLINE", "member_role": "SECONDARY", "member_version": "8.0.19"}, value: 1, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}

func TestScrapePerfReplicationGroupMembersMySQL57(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{
		"CHANNEL_NAME",
		"MEMBER_ID",
		"MEMBER_HOST",
		"MEMBER_PORT",
		"MEMBER_STATE",
	}

	rows := sqlmock.NewRows(columns).
		AddRow("group_replication_applier", "uuid1", "hostname1", "3306", "ONLINE").
		AddRow("group_replication_applier", "uuid2", "hostname2", "3306", "ONLINE").
		AddRow("group_replication_applier", "uuid3", "hostname3", "3306", "ONLINE")

	mock.ExpectQuery(sanitizeQuery(perfReplicationGroupMembersQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapePerfReplicationGroupMembers{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid1", "member_host": "hostname1", "member_port": "3306",
			"member_state": "ONLINE"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid2", "member_host": "hostname2", "member_port": "3306",
			"member_state": "ONLINE"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"channel_name": "group_replication_applier", "member_id": "uuid3", "member_host": "hostname3", "member_port": "3306",
			"member_state": "ONLINE"}, value: 1, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
