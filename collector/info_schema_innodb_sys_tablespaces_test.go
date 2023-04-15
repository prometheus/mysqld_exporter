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
	"github.com/hashicorp/go-version"
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

func TestSemanticVersionCheck(t *testing.T) {
	tests := []struct {
		serverversion    string
		versionthreshold string
		err              bool
	}{
		{"5.5.68", "10.5.0", true},
		{"5.7.42", "10.5.0", true},
		{"8.0.32", "10.5.0", true},
		{"10.0.38", "10.5.0", true},
		{"10.1.48", "10.5.0", true},
		{"10.2.44", "10.5.0", true},
		{"10.3.38", "10.5.0", true},
		{"10.4.28", "10.5.0", true},
		{"10.5.18", "10.5.0", false},
		{"10.6.12", "10.5.0", false},
		{"10.7.8", "10.5.0", false},
		{"10.8.7", "10.5.0", false},
		{"10.9.5", "10.5.0", false},
		{"10.10.3", "10.5.0", false},
		{"10.11.2", "10.5.0", false},
	}

	for _, testcase := range tests {
		var (
			v1, errV1 = version.NewVersion(testcase.serverversion)
			v2, errV2 = version.NewVersion(testcase.versionthreshold)
		)

		if (errV1) != nil {
			t.Errorf("err: serverversion '%s' parsing failed", errV1)
		}

		if (errV2) != nil {
			t.Errorf("err: versionthreshold '%s' parsing failed", errV2)
		}

		testresult := v1.LessThan(v2)
		if testresult != testcase.err {
			t.Errorf("err: semantic version check between '%s' and '%s' failed", testcase.serverversion, testcase.versionthreshold)
			continue
		}
	}
}
