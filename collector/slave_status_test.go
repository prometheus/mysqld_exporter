package collector

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestScrapeSlaveStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	versionColumns := []string{"@@version", "@@version_comment"}
	versionRows := sqlmock.NewRows(versionColumns).
		AddRow("8.0.21", "MySQL Community Server - GPL")
	mock.ExpectQuery(sanitizeQuery("SELECT @@version, @@version_comment")).WillReturnRows(versionRows)

	columns := []string{"Master_Host", "Read_Master_Log_Pos", "Slave_IO_Running", "Slave_SQL_Running", "Seconds_Behind_Master"}
	rows := sqlmock.NewRows(columns).
		AddRow("127.0.0.1", "1", "Connecting", "Yes", "2")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE STATUS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveStatus{}).Scrape(context.Background(), db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 0, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 2, metricType: dto.MetricType_UNTYPED},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestScrapeSlaveStatusVersions(t *testing.T) {

	queryTable := []struct {
		version []driver.Value
		query   string
	}{
		// MariaDB
		{
			version: []driver.Value{"10.5.4-MariaDB-1:10.5.4+maria~focal", "mariadb.org binary distribution"},
			query:   "SHOW ALL SLAVES STATUS",
		},
		{
			version: []driver.Value{"10.4.13-MariaDB-1:10.4.13+maria~focal", "mariadb.org binary distribution"},
			query:   "SHOW ALL SLAVES STATUS",
		},
		{
			version: []driver.Value{"10.3.23-MariaDB-1:10.3.23+maria~focal", "mariadb.org binary distribution"},
			query:   "SHOW ALL SLAVES STATUS",
		},
		{
			version: []driver.Value{"10.2.32-MariaDB-1:10.2.32+maria~bionic", "mariadb.org binary distribution"},
			query:   "SHOW ALL SLAVES STATUS",
		},
		{
			version: []driver.Value{"10.1.45-MariaDB-1~bionic", "mariadb.org binary distribution"},
			query:   "SHOW ALL SLAVES STATUS",
		},
		{
			version: []driver.Value{"5.5.64-MariaDB-1~trusty", "mariadb.org binary distribution"},
			query:   "SHOW SLAVE STATUS",
		},

		// MySQL
		{
			version: []driver.Value{"8.0.21", "MySQL Community Server - GPL"},
			query:   "SHOW SLAVE STATUS",
		},
		{
			version: []driver.Value{"5.7.31", "MySQL Community Server (GPL)"},
			query:   "SHOW SLAVE STATUS",
		},
		{
			version: []driver.Value{"5.6.49", "MySQL Community Server (GPL)"},
			query:   "SHOW SLAVE STATUS",
		},
		{
			version: []driver.Value{"5.5.62", "MySQL Community Server (GPL)"},
			query:   "SHOW SLAVE STATUS",
		},

		// Percona Server
		{
			version: []driver.Value{"8.0.19-10", "Percona Server (GPL), Release 10, Revision f446c04"},
			query:   "SHOW SLAVE STATUS",
		},
		{
			version: []driver.Value{"5.7.30-33", "Percona Server (GPL), Release 33, Revision 6517692"},
			query:   "SHOW SLAVE STATUS",
		},
		{
			version: []driver.Value{"5.6.48-88.0", "Percona Server (GPL), Release 88.0, Revision 66735bc"},
			query:   "SHOW SLAVE STATUS NONBLOCKING",
		},
		{
			version: []driver.Value{"5.5.61-38.13", "Percona Server (GPL), Release 38.13, Revision 812705b"},
			query:   "SHOW SLAVE STATUS NOLOCK",
		},
	}

	for _, qt := range queryTable {

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("error opening a stub database connection: %s", err)
		}
		defer db.Close()

		versionColumns := []string{"@@version", "@@version_comment"}
		versionRows := sqlmock.NewRows(versionColumns).
			AddRow(qt.version...)
		mock.ExpectQuery(sanitizeQuery("SELECT @@version, @@version_comment")).WillReturnRows(versionRows)

		columns := []string{"Master_Host", "Read_Master_Log_Pos", "Slave_IO_Running", "Slave_SQL_Running", "Seconds_Behind_Master"}
		rows := sqlmock.NewRows(columns).
			AddRow("127.0.0.1", "1", "Connecting", "Yes", "2")
		mock.ExpectQuery(sanitizeQuery(qt.query)).WillReturnRows(rows)

		ch := make(chan prometheus.Metric)
		go func() {
			if err = (ScrapeSlaveStatus{}).Scrape(context.Background(), db, ch); err != nil {
				t.Errorf("error calling function on test: %s", err)
			}
			close(ch)
		}()

		counterExpected := []MetricResult{
			{labels: labelMap{"channel_name": "", "connection_name": "", "master_host": "127.0.0.1", "master_uuid": ""}, value: 1, metricType: dto.MetricType_UNTYPED},
		}
		convey.Convey("Metrics comparison", t, func() {
			for _, expect := range counterExpected {
				got := readMetric(<-ch)
				convey.So(got, convey.ShouldResemble, expect)
			}
		})

		// Ensure all SQL queries were executed
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	}
}
