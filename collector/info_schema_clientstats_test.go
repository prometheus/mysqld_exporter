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
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeClientStat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(sanitizeQuery(userstatCheckQuery)).WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("userstat", "ON"))

	columns := []string{"CLIENT", "TOTAL_CONNECTIONS", "CONCURRENT_CONNECTIONS", "CONNECTED_TIME", "BUSY_TIME", "CPU_TIME", "BYTES_RECEIVED", "BYTES_SENT", "BINLOG_BYTES_WRITTEN", "ROWS_READ", "ROWS_SENT", "ROWS_DELETED", "ROWS_INSERTED", "ROWS_UPDATED", "SELECT_COMMANDS", "UPDATE_COMMANDS", "OTHER_COMMANDS", "COMMIT_TRANSACTIONS", "ROLLBACK_TRANSACTIONS", "DENIED_CONNECTIONS", "LOST_CONNECTIONS", "ACCESS_DENIED", "EMPTY_QUERIES"}
	rows := sqlmock.NewRows(columns).
		AddRow("localhost", 1002, 0, 127027, 286, 245, float64(2565104853), 21090856, float64(2380108042), 767691, 1764, 8778, 1210741, 0, 1764, 1214416, 293, 2430888, 0, 0, 0, 0, 0)
	mock.ExpectQuery(sanitizeQuery(clientStatQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeClientStat{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{name: "mysql_info_schema_client_statistics_total_connections", help: "The number of connections created for this client.", labels: labelMap{"client": "localhost"}, value: 1002, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_concurrent_connections", help: "The number of concurrent connections for this client.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_GAUGE},
		{name: "mysql_info_schema_client_statistics_connected_time_seconds_total", help: "The cumulative number of seconds elapsed while there were connections from this client.", labels: labelMap{"client": "localhost"}, value: 127027, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_busy_seconds_total", help: "The cumulative number of seconds there was activity on connections from this client.", labels: labelMap{"client": "localhost"}, value: 286, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_cpu_time_seconds_total", help: "The cumulative CPU time elapsed, in seconds, while servicing this client's connections.", labels: labelMap{"client": "localhost"}, value: 245, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_bytes_received_total", help: "The number of bytes received from this client’s connections.", labels: labelMap{"client": "localhost"}, value: float64(2565104853), metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_bytes_sent_total", help: "The number of bytes sent to this client’s connections.", labels: labelMap{"client": "localhost"}, value: 21090856, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_binlog_bytes_written_total", help: "The number of bytes written to the binary log from this client’s connections.", labels: labelMap{"client": "localhost"}, value: float64(2380108042), metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rows_read_total", help: "The number of rows read by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 767691, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rows_sent_total", help: "The number of rows sent by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 1764, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rows_deleted_total", help: "The number of rows deleted by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 8778, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rows_inserted_total", help: "The number of rows inserted by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 1210741, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rows_updated_total", help: "The number of rows updated by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_select_commands_total", help: "The number of SELECT commands executed from this client’s connections.", labels: labelMap{"client": "localhost"}, value: 1764, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_update_commands_total", help: "The number of UPDATE commands executed from this client’s connections.", labels: labelMap{"client": "localhost"}, value: 1214416, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_other_commands_total", help: "The number of other commands executed from this client’s connections.", labels: labelMap{"client": "localhost"}, value: 293, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_commit_transactions_total", help: "The number of COMMIT commands issued by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 2430888, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_rollback_transactions_total", help: "The number of ROLLBACK commands issued by this client’s connections.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_denied_connections_total", help: "The number of connections denied to this client.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_lost_connections_total", help: "The number of this client’s connections that were terminated uncleanly.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_access_denied_total", help: "The number of times this client’s connections issued commands that were denied.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
		{name: "mysql_info_schema_client_statistics_empty_queries_total", help: "The number of times this client’s connections sent empty queries to the server.", labels: labelMap{"client": "localhost"}, value: 0, metricType: dto.MetricType_COUNTER},
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
