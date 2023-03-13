// Copyright 2021 The Prometheus Authors
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
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeProcesslist(t *testing.T) {
	_, err := kingpin.CommandLine.Parse([]string{
		"--collect.info_schema.processlist.processes_by_user",
		"--collect.info_schema.processlist.processes_by_host",
	})
	if err != nil {
		t.Fatal(err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	query := fmt.Sprintf(infoSchemaProcesslistQuery, 0)
	columns := []string{"user", "host", "command", "state", "processes", "seconds"}
	rows := sqlmock.NewRows(columns).
		AddRow("manager", "10.0.7.234", "Sleep", "", 10, 87).
		AddRow("feedback", "10.0.7.154", "Sleep", "", 8, 842).
		AddRow("root", "10.0.7.253", "Sleep", "", 1, 20).
		AddRow("feedback", "10.0.7.179", "Sleep", "", 2, 14).
		AddRow("system user", "", "Connect", "waiting for handler commit", 1, 7271248).
		AddRow("manager", "10.0.7.234", "Sleep", "", 4, 62).
		AddRow("system user", "", "Query", "Slave has read all relay log; waiting for more updates", 1, 7271248).
		AddRow("event_scheduler", "localhost", "Daemon", "Waiting on empty queue", 1, 7271248)
	mock.ExpectQuery(sanitizeQuery(query)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeProcesslist{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"command": "connect", "state": "waiting_for_handler_commit"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "connect", "state": "waiting_for_handler_commit"}, value: 7271248, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "daemon", "state": "waiting_on_empty_queue"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "daemon", "state": "waiting_on_empty_queue"}, value: 7271248, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "query", "state": "slave_has_read_all_relay_log_waiting_for_more_updates"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "query", "state": "slave_has_read_all_relay_log_waiting_for_more_updates"}, value: 7271248, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "sleep", "state": "unknown"}, value: 25, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"command": "sleep", "state": "unknown"}, value: 1025, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "10.0.7.154"}, value: 8, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "10.0.7.179"}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "10.0.7.234"}, value: 14, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "10.0.7.253"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "localhost"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"client_host": "unknown"}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"mysql_user": "event_scheduler"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"mysql_user": "feedback"}, value: 10, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"mysql_user": "manager"}, value: 14, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"mysql_user": "root"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"mysql_user": "system user"}, value: 2, metricType: dto.MetricType_GAUGE},
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
