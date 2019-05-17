package collector

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
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
		AddRow("Innodb_buffer_pool_pages_dirty", "7").
		AddRow("Innodb_buffer_pool_pages_free", "8").
		AddRow("Innodb_buffer_pool_pages_misc", "9").
		AddRow("Innodb_buffer_pool_pages_old", "10").
		AddRow("Innodb_buffer_pool_pages_total", "11").
		AddRow("Innodb_buffer_pool_pages_flushed", "12").
		AddRow("Innodb_buffer_pool_pages_lru_flushed", "13").
		AddRow("Innodb_buffer_pool_pages_made_not_young", "14").
		AddRow("Innodb_buffer_pool_pages_made_young", "15").
		AddRow("Innodb_rows_read", "16").
		AddRow("Performance_schema_users_lost", "17").
		AddRow("Slave_running", "OFF").
		AddRow("Ssl_version", "").
		AddRow("Uptime", "18").
		AddRow("validate_password.dictionary_file_words_count", "11").
		AddRow("wsrep_cluster_status", "Primary").
		AddRow("wsrep_local_state_uuid", "6c06e583-686f-11e6-b9e3-8336ad58138c").
		AddRow("wsrep_cluster_state_uuid", "6c06e583-686f-11e6-b9e3-8336ad58138c").
		AddRow("wsrep_provider_version", "3.16(r5c765eb)").
		AddRow("wsrep_evs_repl_latency", "0.0471057/0.0722181/0.0783635/0.0112616/6")
	mock.ExpectQuery(sanitizeQuery(globalStatusQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeGlobalStatus{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"command": "alter_db"}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"command": "show_status"}, value: 2, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"command": "select"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"error": "internal"}, value: 4, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"handler": "commit"}, value: 5, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"state": "data"}, value: 6, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "dirty"}, value: 7, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "free"}, value: 8, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "misc"}, value: 9, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "old"}, value: 10, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"state": "total"}, value: 11, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"operation": "flushed"}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"operation": "lru_flushed"}, value: 13, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"operation": "made_not_young"}, value: 14, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"operation": "made_young"}, value: 15, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"operation": "read"}, value: 16, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"instrumentation": "users_lost"}, value: 17, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 18, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 11, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"wsrep_local_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_cluster_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_provider_version": "3.16(r5c765eb)"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"aggregator": "Minimum"}, value: 0.0471057, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"aggregator": "Average"}, value: 0.0722181, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"aggregator": "Maximum"}, value: 0.0783635, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"aggregator": "Standard Deviation"}, value: 0.0112616, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"aggregator": "Sample Size"}, value: 6, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
