package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

type LabelMap map[string]string

type MetricResult struct {
	labels     LabelMap
	value      float64
	metricType dto.MetricType
}

func readMetric(m prometheus.Metric) MetricResult {
	pb := &dto.Metric{}
	m.Write(pb)
	labels := make(LabelMap, len(pb.Label))
	for _, v := range pb.Label {
		labels[v.GetName()] = v.GetValue()
	}
	if pb.Gauge != nil {
		return MetricResult{labels: labels, value: pb.GetGauge().GetValue(), metricType: dto.MetricType_GAUGE}
	}
	if pb.Counter != nil {
		return MetricResult{labels: labels, value: pb.GetCounter().GetValue(), metricType: dto.MetricType_COUNTER}
	}
	if pb.Untyped != nil {
		return MetricResult{labels: labels, value: pb.GetUntyped().GetValue(), metricType: dto.MetricType_UNTYPED}
	}
	panic("Unsupported metric type")
}

func sanitizeQuery(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	q = strings.Replace(q, "(", "\\(", -1)
	q = strings.Replace(q, ")", "\\)", -1)
	return q
}

func Test_scrapeTableStat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"TABLE_SCHEMA", "TABLE_NAME", "ROWS_READ", "ROWS_CHANGED", "ROWS_CHANGED_X_INDEXES"}
	rows := sqlmock.NewRows(columns).
		AddRow("mysql", "db", 238, 0, 8).
		AddRow("mysql", "proxies_priv", 99, 1, 0).
		AddRow("mysql", "user", 1064, 2, 5)
	mock.ExpectQuery(sanitizeQuery(tableStatQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeTableStat(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	expected := []MetricResult{
		{labels: LabelMap{"schema": "mysql", "table": "db"}, value: 238},
		{labels: LabelMap{"schema": "mysql", "table": "db"}, value: 0},
		{labels: LabelMap{"schema": "mysql", "table": "db"}, value: 8},
		{labels: LabelMap{"schema": "mysql", "table": "proxies_priv"}, value: 99},
		{labels: LabelMap{"schema": "mysql", "table": "proxies_priv"}, value: 1},
		{labels: LabelMap{"schema": "mysql", "table": "proxies_priv"}, value: 0},
		{labels: LabelMap{"schema": "mysql", "table": "user"}, value: 1064},
		{labels: LabelMap{"schema": "mysql", "table": "user"}, value: 2},
		{labels: LabelMap{"schema": "mysql", "table": "user"}, value: 5},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func Test_scrapeQueryResponseTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	mock.ExpectQuery(queryResponseCheckQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(1))

	rows := sqlmock.NewRows([]string{"TIME", "COUNT", "TOTAL"}).
		AddRow(0.000001, 124, 0.000000).
		AddRow(0.000010, 179, 0.000797).
		AddRow(0.000100, 2859, 0.107321).
		AddRow(0.001000, 1085, 0.335395).
		AddRow(0.010000, 269, 0.522264).
		AddRow(0.100000, 11, 0.344209).
		AddRow(1.000000, 1, 0.267369).
		AddRow(10.000000, 0, 0.000000).
		AddRow(100.000000, 0, 0.000000).
		AddRow(1000.000000, 0, 0.000000).
		AddRow(10000.000000, 0, 0.000000).
		AddRow(100000.000000, 0, 0.000000).
		AddRow(1000000.000000, 0, 0.000000).
		AddRow("TOO LONG", 0, "TOO LONG")
	mock.ExpectQuery(sanitizeQuery(queryResponseTimeQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeQueryResponseTime(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	// Test counters
	expectTimes := []MetricResult{
		{labels: LabelMap{"le": "1e-06"}, value: 0},
		{labels: LabelMap{"le": "1e-05"}, value: 0.000797},
		{labels: LabelMap{"le": "0.0001"}, value: 0.108118},
		{labels: LabelMap{"le": "0.001"}, value: 0.443513},
		{labels: LabelMap{"le": "0.01"}, value: 0.9657769999999999},
		{labels: LabelMap{"le": "0.1"}, value: 1.3099859999999999},
		{labels: LabelMap{"le": "1"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "10"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "100"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "1000"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "10000"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "100000"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "1e+06"}, value: 1.5773549999999998},
		{labels: LabelMap{"le": "+Inf"}, value: 1.5773549999999998},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expectTimes {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})

	// Test histogram
	expectCounts := map[float64]uint64{
		1e-06:  124,
		1e-05:  303,
		0.0001: 3162,
		0.001:  4247,
		0.01:   4516,
		0.1:    4527,
		1:      4528,
		10:     4528,
		100:    4528,
		1000:   4528,
		10000:  4528,
		100000: 4528,
		1e+06:  4528,
	}
	expectHistogram := prometheus.MustNewConstHistogram(infoSchemaQueryResponseTimeCountDesc,
		4528, 1.5773549999999998, expectCounts)
	expectPb := &dto.Metric{}
	expectHistogram.Write(expectPb)

	gotPb := &dto.Metric{}
	gotHistogram := <-ch // read the last item from channel
	gotHistogram.Write(gotPb)
	convey.Convey("Histogram comparison", t, func() {
		convey.So(expectPb.Histogram, convey.ShouldResemble, gotPb.Histogram)
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func Test_parseMycnf(t *testing.T) {
	const (
		tcpConfig = `
			[client]
			user = root
			password = abc123
		`
		tcpConfig2 = `
			[client]
			user = root
			password = abc123
			port = 3308
		`
		socketConfig = `
			[client]
			user = user
			password = pass
			socket = /var/lib/mysql/mysql.sock
		`
		socketConfig2 = `
			[client]
			user = dude
			password = nopassword
			# host and port will not be used because of socket presence
			host = 1.2.3.4
			port = 3307
			socket = /var/lib/mysql/mysql.sock
		`
		remoteConfig = `
			[client]
			user = dude
			password = nopassword
			host = 1.2.3.4
			port = 3307
		`
		badConfig = `
			[client]
			user = root
		`
		badConfig2 = `
			[client]
			password = abc123
			socket = /var/lib/mysql/mysql.sock
		`
		badConfig3 = `
			[hello]
			world = ismine
		`
		badConfig4 = `
			[hello]
			world
		`
	)
	convey.Convey("Various .my.cnf configurations", t, func() {
		convey.Convey("Local tcp connection", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig))
			convey.So(dsn, convey.ShouldEqual, "root:abc123@tcp(localhost:3306)/")
		})
		convey.Convey("Local tcp connection on non-default port", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig2))
			convey.So(dsn, convey.ShouldEqual, "root:abc123@tcp(localhost:3308)/")
		})
		convey.Convey("Socket connection", func() {
			dsn, _ := parseMycnf([]byte(socketConfig))
			convey.So(dsn, convey.ShouldEqual, "user:pass@unix(/var/lib/mysql/mysql.sock)/")
		})
		convey.Convey("Socket connection ignoring defined host", func() {
			dsn, _ := parseMycnf([]byte(socketConfig2))
			convey.So(dsn, convey.ShouldEqual, "dude:nopassword@unix(/var/lib/mysql/mysql.sock)/")
		})
		convey.Convey("Remote connection", func() {
			dsn, _ := parseMycnf([]byte(remoteConfig))
			convey.So(dsn, convey.ShouldEqual, "dude:nopassword@tcp(1.2.3.4:3307)/")
		})
		convey.Convey("Missed user", func() {
			_, err := parseMycnf([]byte(badConfig))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("Missed password", func() {
			_, err := parseMycnf([]byte(badConfig2))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("No [client] section", func() {
			_, err := parseMycnf([]byte(badConfig3))
			convey.So(err, convey.ShouldNotBeNil)
		})
		convey.Convey("Invalid config", func() {
			_, err := parseMycnf([]byte(badConfig4))
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

func Test_sanitizeTokudbMetric(t *testing.T) {
	samples := map[string]string{
		"loader: number of calls to loader->close() that failed": "loader_number_of_calls_to_loader_close_that_failed",
		"ft: promotion: stopped anyway, after locking the child": "ft_promotion_stopped_anyway_after_locking_the_child",
		"ft: basement nodes deserialized with fixed-keysize":     "ft_basement_nodes_deserialized_with_fixed_keysize",
		"memory: number of bytes used (requested + overhead)":    "memory_number_of_bytes_used_requested_and_overhead",
		"ft: uncompressed / compressed bytes written (overall)":  "ft_uncompressed_and_compressed_bytes_written_overall",
	}
	convey.Convey("Replacement tests", t, func() {
		for metric := range samples {
			got := sanitizeTokudbMetric(metric)
			convey.So(got, convey.ShouldEqual, samples[metric])
		}
	})
}

func Test_scrapeEngineTokudbStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"Type", "Name", "Status"}
	rows := sqlmock.NewRows(columns).
		AddRow("TokuDB", "indexer: number of calls to indexer->build() succeeded", "1").
		AddRow("TokuDB", "ft: promotion: stopped anyway, after locking the child", "45316247").
		AddRow("TokuDB", "memory: mallocator version", "3.3.1-0-g9ef9d9e8c271cdf14f664b871a8f98c827714784").
		AddRow("TokuDB", "filesystem: most recent disk full", "Thu Jan  1 00:00:00 1970").
		AddRow("TokuDB", "locktree: time spent ending the STO early (seconds)", "9115.904484")

	mock.ExpectQuery(sanitizeQuery(engineTokudbStatusQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeEngineTokudbStatus(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricsExpected := []MetricResult{
		{labels: LabelMap{}, value: 1, metricType: dto.MetricType_UNTYPED},
		{labels: LabelMap{}, value: 45316247, metricType: dto.MetricType_UNTYPED},
		{labels: LabelMap{}, value: 9115.904484, metricType: dto.MetricType_UNTYPED},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricsExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func Test_scrapePerfIndexIOWaits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH", "COUNT_INSERT", "COUNT_UPDATE", "COUNT_DELETE", "SUM_TIMER_FETCH", "SUM_TIMER_INSERT", "SUM_TIMER_UPDATE", "SUM_TIMER_DELETE"}
	rows := sqlmock.NewRows(columns).
		// Note, timers are in picoseconds.
		AddRow("database", "table", "index", "10", "11", "12", "13", "14000000000000", "15000000000000", "16000000000000", "17000000000000").
		AddRow("database", "table", "NONE", "20", "21", "22", "23", "24000000000000", "25000000000000", "26000000000000", "27000000000000")
	mock.ExpectQuery(sanitizeQuery(perfIndexIOWaitsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapePerfIndexIOWaits(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "fetch"}, value: 10, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "update"}, value: 12, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "delete"}, value: 13, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "fetch"}, value: 14, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "update"}, value: 16, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "index", "operation": "delete"}, value: 17, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "fetch"}, value: 20, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "insert"}, value: 21, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "update"}, value: 22, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "delete"}, value: 23, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "fetch"}, value: 24, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "insert"}, value: 25, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "update"}, value: 26, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"schema": "database", "name": "table", "index": "NONE", "operation": "delete"}, value: 27, metricType: dto.MetricType_COUNTER},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func Test_scrapeInnodbMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	columns := []string{"name", "subsystem", "type", "comment", "count"}
	rows := sqlmock.NewRows(columns).
		AddRow("lock_timeouts", "lock", "counter", "Number of lock timeouts", 0).
		AddRow("buffer_pool_reads", "buffer", "status_counter", "Number of reads directly from disk (innodb_buffer_pool_reads)", 1).
		AddRow("buffer_pool_size", "server", "value", "Server buffer pool size (all buffer pools) in bytes", 2).
		AddRow("buffer_page_read_system_page", "buffer_page_io", "counter", "Number of System Pages read", 3).
		AddRow("buffer_page_written_undo_log", "buffer_page_io", "counter", "Number of Undo Log Pages written", 4)
	mock.ExpectQuery(sanitizeQuery(infoSchemaInnodbMetricsQuery)).WillReturnRows(rows)

	ch := make(chan prometheus.Metric)
	go func() {
		if err = scrapeInnodbMetrics(db, ch); err != nil {
			t.Errorf("error calling function on test: %s", err)
		}
		close(ch)
	}()

	metricExpected := []MetricResult{
		{labels: LabelMap{}, value: 0, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{}, value: 1, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{}, value: 2, metricType: dto.MetricType_GAUGE},
		{labels: LabelMap{"type": "system_page"}, value: 3, metricType: dto.MetricType_COUNTER},
		{labels: LabelMap{"type": "undo_log"}, value: 4, metricType: dto.MetricType_COUNTER},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range metricExpected {
			got := readMetric(<-ch)
			convey.So(got, convey.ShouldResemble, expect)
		}
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
