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
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeSlaveHostsOldFormat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	columns := []string{"Server_id", "Host", "Port", "Rpl_recovery_rank", "Master_id"}
	rows := sqlmock.NewRows(columns).
		AddRow("380239978", "backup_server_1", "0", "1", "192168011").
		AddRow("11882498", "backup_server_2", "0", "1", "192168011")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"server_id": "380239978", "slave_host": "backup_server_1", "port": "0", "master_id": "192168011", "slave_uuid": ""}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "11882498", "slave_host": "backup_server_2", "port": "0", "master_id": "192168011", "slave_uuid": ""}, value: 1, metricType: dto.MetricType_GAUGE},
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

func TestScrapeSlaveHostsNewFormat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	columns := []string{"Server_id", "Host", "Port", "Master_id", "Slave_UUID"}
	rows := sqlmock.NewRows(columns).
		AddRow("192168010", "iconnect2", "3306", "192168011", "14cb6624-7f93-11e0-b2c0-c80aa9429562").
		AddRow("1921680101", "athena", "3306", "192168011", "07af4990-f41f-11df-a566-7ac56fdaf645")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"server_id": "192168010", "slave_host": "iconnect2", "port": "3306", "master_id": "192168011", "slave_uuid": "14cb6624-7f93-11e0-b2c0-c80aa9429562"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "1921680101", "slave_host": "athena", "port": "3306", "master_id": "192168011", "slave_uuid": "07af4990-f41f-11df-a566-7ac56fdaf645"}, value: 1, metricType: dto.MetricType_GAUGE},
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

func TestScrapeSlaveHostsWithoutSlaveUuid(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	columns := []string{"Server_id", "Host", "Port", "Master_id"}
	rows := sqlmock.NewRows(columns).
		AddRow("192168010", "iconnect2", "3306", "192168012").
		AddRow("1921680101", "athena", "3306", "192168012")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"server_id": "192168010", "slave_host": "iconnect2", "port": "3306", "master_id": "192168012", "slave_uuid": ""}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"server_id": "1921680101", "slave_host": "athena", "port": "3306", "master_id": "192168012", "slave_uuid": ""}, value: 1, metricType: dto.MetricType_GAUGE},
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

func TestScrapeSlaveHostsPermissionErrorNotMasked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	// Permission denied on SHOW SLAVE HOSTS must be returned as-is.
	// Falling back to SHOW REPLICAS would produce a misleading syntax error
	// on MariaDB / MySQL versions that do not support that statement.
	permErr := &mysql.MySQLError{
		Number:  1227,
		Message: "Access denied; you need (at least one of) the REPLICATION SLAVE privilege(s) for this operation",
	}
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnError(permErr)

	ch := make(chan prometheus.Metric)
	scrapeErr := (ScrapeSlaveHosts{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger())
	close(ch)

	if scrapeErr == nil {
		t.Fatal("expected permission error, got nil")
	}
	var mysqlErr *mysql.MySQLError
	if !errors.As(scrapeErr, &mysqlErr) {
		t.Fatalf("expected *mysql.MySQLError, got %T: %v", scrapeErr, scrapeErr)
	}
	if mysqlErr.Number != 1227 {
		t.Fatalf("expected MySQL error 1227, got %d: %v", mysqlErr.Number, scrapeErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}

func TestScrapeSlaveHostsFallbackOnSyntaxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	// SHOW SLAVE HOSTS is gone on MySQL 8.4+; fall back to SHOW REPLICAS.
	syntaxErr := &mysql.MySQLError{
		Number:  1064,
		Message: "You have an error in your SQL syntax; check the manual that corresponds to your MySQL server version for the right syntax to use near 'SLAVE HOSTS'",
	}
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnError(syntaxErr)

	columns := []string{"Server_id", "Host", "Port", "Master_id", "Replica_UUID"}
	rows := sqlmock.NewRows(columns).
		AddRow("192168010", "iconnect2", "3306", "192168011", "14cb6624-7f93-11e0-b2c0-c80aa9429562")
	mock.ExpectQuery(sanitizeQuery("SHOW REPLICAS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	counterExpected := []MetricResult{
		{labels: labelMap{"server_id": "192168010", "slave_host": "iconnect2", "port": "3306", "master_id": "192168011", "slave_uuid": "14cb6624-7f93-11e0-b2c0-c80aa9429562"}, value: 1, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range counterExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}
