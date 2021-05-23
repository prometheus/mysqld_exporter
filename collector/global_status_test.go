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

func TestScrapeGlobalStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Variable_name", "Value"}
	rows := sqlmock.NewRows(columns).
		AddRow("Com_alter_db", "1").
		AddRow("Com_show_status", "2").
		AddRow("Com_select", "3").
		AddRow("Connection_errors_internal", "4").
		AddRow("Handler_commit", "5").
		AddRow("Innodb_buffer_pool_pages_data", "6").
		AddRow("Innodb_buffer_pool_pages_flushed", "7").
		AddRow("Innodb_buffer_pool_pages_dirty", "7").
		AddRow("Innodb_buffer_pool_pages_free", "8").
		AddRow("Innodb_buffer_pool_pages_misc", "9").
		AddRow("Innodb_buffer_pool_pages_old", "10").
		AddRow("Innodb_buffer_pool_pages_total", "11").
		AddRow("Innodb_buffer_pool_pages_lru_flushed", "13").
		AddRow("Innodb_buffer_pool_pages_made_not_young", "14").
		AddRow("Innodb_buffer_pool_pages_made_young", "15").
		AddRow("Innodb_rows_read", "8").
		AddRow("Performance_schema_users_lost", "9").
		AddRow("Slave_running", "OFF").
		AddRow("Ssl_version", "").
		AddRow("Uptime", "10").
		AddRow("validate_password.dictionary_file_words_count", "11").
		AddRow("wsrep_cluster_status", "Primary").
		AddRow("wsrep_local_state_uuid", "6c06e583-686f-11e6-b9e3-8336ad58138c").
		AddRow("wsrep_cluster_state_uuid", "6c06e583-686f-11e6-b9e3-8336ad58138c").
		AddRow("wsrep_provider_version", "3.16(r5c765eb)").
		AddRow("wsrep_evs_repl_latency", "0.000227664/0.00034135/0.000544298/6.03708e-05/212")
	mock.ExpectQuery(sanitizeQuery(globalStatusQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeGlobalStatus{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{name: "mysql_global_status_commands_total", help: "Total number of executed MySQL commands.", labels: labelMap{"command": "alter_db"}, value: 1, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_commands_total", help: "Total number of executed MySQL commands.", labels: labelMap{"command": "show_status"}, value: 2, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_commands_total", help: "Total number of executed MySQL commands.", labels: labelMap{"command": "select"}, value: 3, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_connection_errors_total", help: "Total number of MySQL connection errors.", labels: labelMap{"error": "internal"}, value: 4, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_handlers_total", help: "Total number of executed MySQL handlers.", labels: labelMap{"handler": "commit"}, value: 5, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_buffer_pool_pages", help: "Innodb buffer pool pages by state.", labels: labelMap{"state": "data"}, value: 6, metricType: dto.MetricType_GAUGE},
		{name: "mysql_global_status_buffer_pool_page_changes_total", help: "Innodb buffer pool page state changes.", labels: labelMap{"operation": "flushed"}, value: 7, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_buffer_pool_dirty_pages", help: "Innodb buffer pool dirty pages.", labels: labelMap{}, value: 7, metricType: dto.MetricType_GAUGE},
		{name: "mysql_global_status_buffer_pool_pages", help: "Innodb buffer pool pages by state.", labels: labelMap{"state": "free"}, value: 8, metricType: dto.MetricType_GAUGE},
		{name: "mysql_global_status_buffer_pool_pages", help: "Innodb buffer pool pages by state.", labels: labelMap{"state": "misc"}, value: 9, metricType: dto.MetricType_GAUGE},
		{name: "mysql_global_status_buffer_pool_pages", help: "Innodb buffer pool pages by state.", labels: labelMap{"state": "old"}, value: 10, metricType: dto.MetricType_GAUGE},
		//{labels: labelMap{"state": "total_pages"}, value: 11, metricType: dto.MetricType_GAUGE},
		{name: "mysql_global_status_buffer_pool_page_changes_total", help: "Innodb buffer pool page state changes.", labels: labelMap{"operation": "lru_flushed"}, value: 13, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_buffer_pool_page_changes_total", help: "Innodb buffer pool page state changes.", labels: labelMap{"operation": "made_not_young"}, value: 14, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_buffer_pool_page_changes_total", help: "Innodb buffer pool page state changes.", labels: labelMap{"operation": "made_young"}, value: 15, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_innodb_row_ops_total", help: "Total number of MySQL InnoDB row operations.", labels: labelMap{"operation": "read"}, value: 8, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_performance_schema_lost_total", help: "Total number of MySQL instrumentations that could not be loaded or created due to memory constraints.", labels: labelMap{"instrumentation": "users_lost"}, value: 9, metricType: dto.MetricType_COUNTER},
		{name: "mysql_global_status_slave_running", help: "Generic metric from SHOW GLOBAL STATUS.", labels: labelMap{}, value: 0, metricType: dto.MetricType_UNTYPED},
		{name: "mysql_global_status_uptime", help: "Generic metric from SHOW GLOBAL STATUS.", labels: labelMap{}, value: 10, metricType: dto.MetricType_UNTYPED},
		{name: "mysql_global_status_validate_password_dictionary_file_words_count", help: "Generic metric from SHOW GLOBAL STATUS.", labels: labelMap{}, value: 11, metricType: dto.MetricType_UNTYPED},
		{name: "mysql_global_status_wsrep_cluster_status", help: "Generic metric from SHOW GLOBAL STATUS.", labels: labelMap{}, value: 1, metricType: dto.MetricType_UNTYPED},
		{name: "mysql_galera_status_info", help: "PXC/Galera status information.", labels: labelMap{"wsrep_local_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_cluster_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_provider_version": "3.16(r5c765eb)"}, value: 1, metricType: dto.MetricType_GAUGE},
		{name: "mysql_galera_evs_repl_latency_min_seconds", help: "PXC/Galera group communication latency. Min value.", labels: labelMap{}, value: 0.000227664, metricType: dto.MetricType_GAUGE},
		{name: "mysql_galera_evs_repl_latency_avg_seconds", help: "PXC/Galera group communication latency. Avg value.", labels: labelMap{}, value: 0.00034135, metricType: dto.MetricType_GAUGE},
		{name: "mysql_galera_evs_repl_latency_max_seconds", help: "PXC/Galera group communication latency. Max value.", labels: labelMap{}, value: 0.000544298, metricType: dto.MetricType_GAUGE},
		{name: "mysql_galera_evs_repl_latency_stdev", help: "PXC/Galera group communication latency. Standard Deviation.", labels: labelMap{}, value: 6.03708e-05, metricType: dto.MetricType_GAUGE},
		{name: "mysql_galera_evs_repl_latency_sample_size", help: "PXC/Galera group communication latency. Sample Size.", labels: labelMap{}, value: 212, metricType: dto.MetricType_GAUGE},
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
