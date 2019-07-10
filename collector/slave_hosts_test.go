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
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeSlaveHostsOldFormat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Server_id", "Host", "Port", "Rpl_recovery_rank", "Master_id"}
	rows := sqlmock.NewRows(columns).
		AddRow("380239978", "backup_server_1", "0", "1", "192168011").
		AddRow("11882498", "backup_server_2", "0", "1", "192168011")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), db, ch); err != nil {
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

	columns := []string{"Server_id", "Host", "Port", "Master_id", "Slave_UUID"}
	rows := sqlmock.NewRows(columns).
		AddRow("192168010", "iconnect2", "3306", "192168011", "14cb6624-7f93-11e0-b2c0-c80aa9429562").
		AddRow("1921680101", "athena", "3306", "192168011", "07af4990-f41f-11df-a566-7ac56fdaf645")
	mock.ExpectQuery(sanitizeQuery("SHOW SLAVE HOSTS")).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeSlaveHosts{}).Scrape(context.Background(), db, ch); err != nil {
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
