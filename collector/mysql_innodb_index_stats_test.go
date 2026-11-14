// Copyright The Prometheus Authors
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
	"database/sql"
	"errors"
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
	// Like MariaDB and MySQL before 8.0: no INNODB_TABLES, so INNODB_SYS_TABLES is used.
	mock.ExpectQuery(regexp.QuoteMeta(innodbCompressedTablesQueries[0])).
		WillReturnError(errors.New("Unknown table 'INNODB_TABLES' in information_schema"))
	mock.ExpectQuery(regexp.QuoteMeta(innodbCompressedTablesQueries[1])).
		WillReturnRows(sqlmock.NewRows([]string{"NAME", "ZIP_PAGE_SIZE"}).AddRow("shop/archive", 8192))

	columns := []string{"database_name", "table_name", "index_name", "stat_name", "stat_value", "stat_description", "last_update"}
	rows := sqlmock.NewRows(columns).
		AddRow("shop", "orders", "PRIMARY", "n_diff_pfx01", 1000, "id", 1700000000).
		AddRow("shop", "orders", "PRIMARY", "n_leaf_pages", 10, "Number of leaf pages in the index", 1700000000).
		AddRow("shop", "orders", "PRIMARY", "size", 11, "Number of pages in the index", 1700000000).
		AddRow("shop", "orders", "idx_customer", "n_diff_pfx01", 50, "customer_id", 1700000100).
		AddRow("shop", "orders", "idx_customer", "n_diff_pfx02", 1000, "customer_id,id", 1700000100).
		AddRow("shop", "orders", "idx_customer", "n_leaf_pages", 2, "Number of leaf pages in the index", 1700000100).
		AddRow("shop", "orders", "idx_customer", "size", 3, "Number of pages in the index", 1700000100).
		// A compressed table, and two prefixes whose descriptions MySQL truncated to the same text
		AddRow("shop", "archive", "PRIMARY", "n_diff_pfx16", 70, "c01,c02,c03", 1700000200).
		AddRow("shop", "archive", "PRIMARY", "n_diff_pfx17", 80, "c01,c02,c03", 1700000200).
		AddRow("shop", "archive", "PRIMARY", "size", 4, "Number of pages in the index", 1700000200)
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
	archive := labelMap{"schema": "shop", "table": "archive", "index": "PRIMARY"}
	withColumns := func(labels labelMap, prefix, columns string) labelMap {
		out := labelMap{"prefix": prefix, "columns": columns}
		for k, v := range labels {
			out[k] = v
		}
		return out
	}

	expected := []MetricResult{
		{labels: withColumns(primary, "1", "id"), value: 1000, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 10 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 11 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: primary, value: 1700000000, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(secondary, "1", "customer_id"), value: 50, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(secondary, "2", "customer_id,id"), value: 1000, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 2 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 3 * 16384, metricType: dto.MetricType_GAUGE},
		{labels: secondary, value: 1700000100, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(archive, "16", "c01,c02,c03"), value: 70, metricType: dto.MetricType_GAUGE},
		{labels: withColumns(archive, "17", "c01,c02,c03"), value: 80, metricType: dto.MetricType_GAUGE},
		{labels: archive, value: 4 * 8192, metricType: dto.MetricType_GAUGE},
		{labels: archive, value: 1700000200, metricType: dto.MetricType_GAUGE},
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
		"mysql_innodb_index_stats_distinct_values",
		"mysql_innodb_index_stats_distinct_values",
		"mysql_innodb_index_stats_size_bytes",
		"mysql_innodb_index_stats_last_update_timestamp_seconds",
	}

	var got []MetricResult
	var gotNames []string
	var metrics replayCollector
	for m := range ch {
		got = append(got, readMetric(m))
		gotNames = append(gotNames, metricName(m))
		metrics = append(metrics, m)
	}
	// A registry rejects duplicate series, which would fail the whole scrape.
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics)
	if _, err := registry.Gather(); err != nil {
		t.Errorf("gathering the metrics failed: %s", err)
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

// replayCollector sends metrics that were already collected, so they can be
// checked through a registry.
type replayCollector []prometheus.Metric

func (replayCollector) Describe(chan<- *prometheus.Desc) {}

func (c replayCollector) Collect(ch chan<- prometheus.Metric) {
	for _, m := range c {
		ch <- m
	}
}

func TestInnodbIndexStatsFilter(t *testing.T) {
	for _, tc := range []struct {
		databases  string
		wantFilter string
		wantArgs   []any
		wantErr    bool
	}{
		{"*", innodbIndexStatsSystemSchemasFilter, nil, false},
		{"shop", "database_name IN (?)", []any{"shop"}, false},
		{" shop, billing ,", "database_name IN (?, ?)", []any{"shop", "billing"}, false},
		// A list with no names must not turn into "every database"
		{",", "", nil, true},
		{"  ", "", nil, true},
	} {
		filter, args, err := innodbIndexStatsFilter(tc.databases)
		if (err != nil) != tc.wantErr {
			t.Errorf("databases %q: err = %v, want error %v", tc.databases, err, tc.wantErr)
		}
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
	analyzeTable(ctx, t, db, "mysqld_exporter_test.innodb_index_stats")

	// A compressed table, whose pages are KEY_BLOCK_SIZE and not innodb_page_size.
	// Servers without Barracuda (MySQL 5.6 defaults) can't create it.
	compressed := true
	for _, query := range []string{
		"DROP TABLE IF EXISTS mysqld_exporter_test.innodb_index_stats_zip",
		"CREATE TABLE mysqld_exporter_test.innodb_index_stats_zip (id INT PRIMARY KEY) ENGINE=InnoDB STATS_PERSISTENT=1 ROW_FORMAT=COMPRESSED KEY_BLOCK_SIZE=8",
		"INSERT INTO mysqld_exporter_test.innodb_index_stats_zip VALUES (1), (2), (3)",
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Logf("skipping the compressed table check: %s", err)
			compressed = false
			break
		}
	}
	if compressed {
		analyzeTable(ctx, t, db, "mysqld_exporter_test.innodb_index_stats_zip")
	}

	ch := make(chan prometheus.Metric)
	errCh := make(chan error, 1)
	go func() {
		scraper := ScrapeInnodbIndexStats(config.InnodbIndexStatsConfig{Databases: "mysqld_exporter_test"})
		errCh <- scraper.Scrape(ctx, inst, ch, promslog.NewNopLogger())
		close(ch)
	}()

	sizes := map[string]float64{}
	zipSizes := map[string]float64{}
	distinct := map[string]bool{}
	for m := range ch {
		result := readMetric(m)
		if result.labels["table"] == "innodb_index_stats_zip" && metricName(m) == "mysql_innodb_index_stats_size_bytes" {
			zipSizes[result.labels["index"]] = result.value
		}
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

	if !compressed {
		return
	}
	pageSize, ok := innodbCompressedPageSizes(ctx, db, promslog.NewNopLogger())["mysqld_exporter_test/innodb_index_stats_zip"]
	if !ok {
		t.Log("the server didn't compress the table, skipping the compressed size check")
		return
	}
	var pages float64
	if err := db.QueryRowContext(ctx, `SELECT stat_value FROM mysql.innodb_index_stats
		WHERE database_name = 'mysqld_exporter_test' AND table_name = 'innodb_index_stats_zip'
		AND index_name = 'PRIMARY' AND stat_name = 'size'`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if want := pages * pageSize; zipSizes["PRIMARY"] != want {
		t.Errorf("compressed table size = %v, want %v pages of %v bytes", zipSizes["PRIMARY"], pages, pageSize)
	}
}

// analyzeTable runs ANALYZE TABLE, which returns a result set, so it's read with Query.
func analyzeTable(ctx context.Context, t *testing.T, db *sql.DB, table string) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "ANALYZE TABLE "+table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	analyzed := 0
	for rows.Next() {
		analyzed++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if analyzed == 0 {
		t.Fatalf("ANALYZE TABLE %s returned no rows", table)
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
