// Copyright 2026 The Prometheus Authors
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
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"

	"github.com/prometheus/mysqld_exporter/config"
)

func innodbIndexStatsQueryRegexp(filter string) string {
	query := strings.Replace(innodbIndexStatsQuery, "%s", filter, 1)
	return "^" + regexp.QuoteMeta(strings.Join(strings.Fields(query), " ")) + "$"
}

func TestScrapeInnodbIndexStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()
	inst := &instance{db: db}

	mock.ExpectQuery(regexp.QuoteMeta(innodbPageSizeQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"@@innodb_page_size"}).AddRow(16384))

	columns := []string{"database_name", "table_name", "index_name", "stat_name", "stat_value", "stat_description", "last_update"}
	rows := sqlmock.NewRows(columns).
		AddRow("shop", "orders", "PRIMARY", "n_diff_pfx01", 1000, "id", 1700000000).
		AddRow("shop", "orders", "PRIMARY", "n_leaf_pages", 10, "Number of leaf pages in the index", 1700000000).
		AddRow("shop", "orders", "PRIMARY", "size", 11, "Number of pages in the index", 1700000000).
		AddRow("shop", "orders", "idx_customer", "n_diff_pfx01", 50, "customer_id", 1700000100).
		AddRow("shop", "orders", "idx_customer", "n_diff_pfx02", 1000, "customer_id,id", 1700000100).
		AddRow("shop", "orders", "idx_customer", "n_leaf_pages", 2, "Number of leaf pages in the index", 1700000100).
		AddRow("shop", "orders", "idx_customer", "size", 3, "Number of pages in the index", 1700000100)
	mock.ExpectQuery(innodbIndexStatsQueryRegexp("database_name IN (?)")).
		WithArgs("shop").
		WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		scraper := ScrapeInnodbIndexStats(config.InnodbIndexStatsConfig{Databases: "shop"})
		if err := scraper.Scrape(context.Background(), inst, ch, promslog.NewNopLogger()); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	primary := labelMap{"schema": "shop", "table": "orders", "index": "PRIMARY"}
	secondary := labelMap{"schema": "shop", "table": "orders", "index": "idx_customer"}
	withColumns := func(labels labelMap, columns string) labelMap {
		out := labelMap{"columns": columns}
		for k, v := range labels {
			out[k] = v
		}
		return out
	}

	expected := []MetricResult{
		{labels: withColumns(primary, "id"), value: 1000, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 10 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 11 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 1700000000, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(secondary, "customer_id"), value: 50, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(secondary, "customer_id,id"), value: 1000, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 2 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 3 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 1700000100, metricType: dto.MetricType_GAUGE},
	}
	expectedNames := []string{
		"mysql_innodb_index_stats_distinct_values",
		"mysql_innodb_index_stats_leaf_size_bytes",
		"mysql_innodb_index_stats_size_bytes",
		"mysql_innodb_index_stats_last_update_timestamp_seconds",
		"mysql_innodb_index_stats_distinct_values",
		"mysql_innodb_index_stats_distinct_values",
		"mysql_innodb_index_stats_leaf_size_bytes",
		"mysql_innodb_index_stats_size_bytes",
		"mysql_innodb_index_stats_last_update_timestamp_seconds",
	}

	var got []MetricResult
	var gotNames []string
	for m := range ch {
		got = append(got, readMetric(m))
		gotNames = append(gotNames, metricName(m))
	}
	if diff := cmp.Diff(expected, got, cmp.AllowUnexported(MetricResult{})); diff != "" {
		t.Errorf("unexpected metrics (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(expectedNames, gotNames); diff != "" {
		t.Errorf("unexpected metric names (-want +got):\n%s", diff)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled exceptions: %s", err)
	}
}

func TestInnodbIndexStatsFilter(t *testing.T) {
	for _, tc := range []struct {
		databases  string
		wantFilter string
		wantArgs   []any
	}{
		{"*", innodbIndexStatsSystemSchemasFilter, nil},
		{"shop", "database_name IN (?)", []any{"shop"}},
		{" shop, billing ,", "database_name IN (?, ?)", []any{"shop", "billing"}},
		{",", innodbIndexStatsSystemSchemasFilter, nil},
	} {
		filter, args := innodbIndexStatsFilter(tc.databases)
		if filter != tc.wantFilter {
			t.Errorf("databases %q: filter = %q, want %q", tc.databases, filter, tc.wantFilter)
		}
		if diff := cmp.Diff(tc.wantArgs, args); diff != "" {
			t.Errorf("databases %q: unexpected args (-want +got):\n%s", tc.databases, diff)
		}
	}
}

// TestScrapeInnodbIndexStatsLive runs the collector against a real server, so
// the integration job checks every MySQL, Percona and MariaDB version it tests.
func TestScrapeInnodbIndexStatsLive(t *testing.T) {
	connDSN := os.Getenv("TEST_MYSQL_DSN")
	if connDSN == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	inst, err := newInstance(ctx, connDSN, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	db := inst.getDB()
	for _, query := range []string{
		"CREATE DATABASE IF NOT EXISTS mysqld_exporter_test",
		"DROP TABLE IF EXISTS mysqld_exporter_test.innodb_index_stats",
		"CREATE TABLE mysqld_exporter_test.innodb_index_stats (id INT PRIMARY KEY, customer_id INT, KEY idx_customer (customer_id)) ENGINE=InnoDB STATS_PERSISTENT=1",
		"INSERT INTO mysqld_exporter_test.innodb_index_stats VALUES (1, 10), (2, 10), (3, 20), (4, 30)",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	// ANALYZE TABLE returns a result set, so read it with Query.
	analyzeRows, err := db.QueryContext(ctx, "ANALYZE TABLE mysqld_exporter_test.innodb_index_stats")
	if err != nil {
		t.Fatal(err)
	}
	analyzed := 0
	for analyzeRows.Next() {
		analyzed++
	}
	if err := analyzeRows.Err(); err != nil {
		t.Fatal(err)
	}
	analyzeRows.Close()
	if analyzed == 0 {
		t.Fatal("ANALYZE TABLE returned no rows")
	}

	ch := make(chan prometheus.Metric)
	errCh := make(chan error, 1)
	go func() {
		scraper := ScrapeInnodbIndexStats(config.InnodbIndexStatsConfig{Databases: "mysqld_exporter_test"})
		errCh <- scraper.Scrape(ctx, inst, ch, promslog.NewNopLogger())
		close(ch)
	}()

	sizes := map[string]float64{}
	distinct := map[string]bool{}
	for m := range ch {
		result := readMetric(m)
		if result.labels["table"] != "innodb_index_stats" {
			continue
		}
		switch metricName(m) {
		case "mysql_innodb_index_stats_size_bytes":
			sizes[result.labels["index"]] = result.value
		case "mysql_innodb_index_stats_distinct_values":
			distinct[result.labels["index"]+"/"+result.labels["columns"]] = true
		}
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	for _, index := range []string{"PRIMARY", "idx_customer"} {
		if sizes[index] <= 0 {
			t.Errorf("expected a positive size for index %s, got %v (all sizes: %v)", index, sizes[index], sizes)
		}
	}
	if !distinct["idx_customer/customer_id"] {
		t.Errorf("expected distinct values for idx_customer/customer_id, got %v", distinct)
	}
}

// metricName returns the fully qualified name of a metric.
func metricName(m prometheus.Metric) string {
	desc := m.Desc().String()
	const prefix = `fqName: "`
	start := strings.Index(desc, prefix)
	if start < 0 {
		return ""
	}
	desc = desc[start+len(prefix):]
	end := strings.Index(desc, `"`)
	if end < 0 {
		return ""
	}
	return desc[:end]
}
