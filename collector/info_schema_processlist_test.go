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
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestProcesslistQueryOnlyFullGroupBy(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Both sql_mode and SHOW WARNINGS are local to the session.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// #684: Enable strict grouping explicitly so the test catches the MariaDB even on servers where ONLY_FULL_GROUP_BY is disabled by default.
	if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = 'ONLY_FULL_GROUP_BY'"); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(infoSchemaProcesslistQuery, 0)); err != nil {
		t.Fatalf("processlist query: %v", err)
	}

	// Ambiguous GROUP BY columns can produce warning 1052 even when the query succeeds.
	// Read its warnings before another statement changes the session diagnostics.
	warnings, err := conn.QueryContext(ctx, "SHOW WARNINGS")
	if err != nil {
		t.Fatal(err)
	}
	defer warnings.Close()

	for warnings.Next() {
		var level, message string
		var code int
		if err := warnings.Scan(&level, &code, &message); err != nil {
			t.Fatal(err)
		}
		if code == 1052 {
			t.Errorf("processlist query: %s %d: %s", level, code, message)
		}
	}
	if err := warnings.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestScrapeProcesslist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

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
		scraper := ScrapeProcesslist{
			ProcessesByUser: true,
			ProcessesByHost: true,
		}
		if err = scraper.Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
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
