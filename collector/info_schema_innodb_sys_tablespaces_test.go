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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

func TestScrapeInfoSchemaInnodbTablespaces(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("INNODB_SYS_TABLESPACES")
	mock.ExpectQuery(sanitizeQuery(innodbTablespacesTablenameQuery)).WillReturnRows(rows)

	tablespacesTablename := "INNODB_SYS_TABLESPACES"
	columns = []string{"SPACE", "NAME", "FILE_FORMAT", "ROW_FORMAT", "SPACE_TYPE", "FILE_SIZE", "ALLOCATED_SIZE"}
	rows = sqlmock.NewRows(columns).
		AddRow(1, "sys/sys_config", "Barracuda", "Dynamic", "Single", 100, 100).
		AddRow(2, "db/compressed", "Barracuda", "Compressed", "Single", 300, 200)
	query := fmt.Sprintf(innodbTablespacesQuery, tablespacesTablename, tablespacesTablename)
	mock.ExpectQuery(sanitizeQuery(query)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = (ScrapeInfoSchemaInnodbTablespaces{}).Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: labelMap{"tablespace_name": "sys/sys_config", "file_format": "Barracuda", "row_format": "Dynamic", "space_type": "Single"}, value: 1, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "sys/sys_config"}, value: 100, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "sys/sys_config"}, value: 100, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed", "file_format": "Barracuda", "row_format": "Compressed", "space_type": "Single"}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed"}, value: 300, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"tablespace_name": "db/compressed"}, value: 200, metricType: dto.MetricType_GAUGE},
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
