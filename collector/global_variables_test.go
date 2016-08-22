package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeGlobalVariables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Variable_name", "Value"}
	rows := sqlmock.NewRows(columns).
		AddRow("wait_timeout", "28800").
		AddRow("version_compile_os", "Linux").
		AddRow("userstat", "OFF").
		AddRow("transaction_prealloc_size", "4096").
		AddRow("tx_isolation", "REPEATABLE-READ").
		AddRow("tmp_table_size", "16777216").
		AddRow("tmpdir", "/tmp").
		AddRow("sync_binlog", "0").
		AddRow("sync_frm", "ON").
		AddRow("slow_launch_time", "2").
		AddRow("innodb_version", "5.6.30-76.3").
		AddRow("version", "5.6.30-76.3-56").
		AddRow("version_comment", "Percona XtraDB Cluster...").
		AddRow("wsrep_cluster_name", "supercluster")
	mock.ExpectQuery(globalVariablesQuery).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = ScrapeGlobalVariables(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{}, value: 28800, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 4096, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 16777216, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 0, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"innodb_version": "5.6.30-76.3", "version": "5.6.30-76.3-56", "version_comment": "Percona XtraDB Cluster..."}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"wsrep_cluster_name": "supercluster"}, value: 1, metricType: dto.MetricType_GAUGE},
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
