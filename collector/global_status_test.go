package collector

import (
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
		AddRow("Innodb_buffer_pool_pages_flushed", "7").
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
		if err = (ScrapeGlobalStatus{}).Scrape(db, ch); err != nil {
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
		{labels: labelMap{"operation": "flushed"}, value: 7, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"operation": "read"}, value: 8, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{"instrumentation": "users_lost"}, value: 9, metricType: dto.MetricType_COUNTER},
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 10, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 11, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"wsrep_local_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_cluster_state_uuid": "6c06e583-686f-11e6-b9e3-8336ad58138c", "wsrep_provider_version": "3.16(r5c765eb)"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 0.000227664, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 0.00034135, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 0.000544298, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 6.03708e-05, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 212, metricType: dto.MetricType_GAUGE},
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
