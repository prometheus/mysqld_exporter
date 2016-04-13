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
