package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func readCounter(m prometheus.Metric) (map[string]string, float64) {
	pb := &dto.Metric{}
	m.Write(pb)
	labels := make(map[string]string, len(pb.Label))
	for _, v := range pb.Label {
		labels[v.GetName()] = v.GetValue()
	}
	value := pb.GetCounter().GetValue()
	return labels, value
}

func sanitizeQuery(q string) string {
	return strings.Join(strings.Fields(q), " ")
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

	// Test counters one by one to easy spot a mismatch
	expectTimes := map[string]float64{
		"1e-06":  0,
		"1e-05":  0.000797,
		"0.0001": 0.108118,
		"0.001":  0.443513,
		"0.01":   0.9657769999999999,
		"0.1":    1.3099859999999999,
		"1":      1.5773549999999998,
		"10":     1.5773549999999998,
		"100":    1.5773549999999998,
		"1000":   1.5773549999999998,
		"10000":  1.5773549999999998,
		"100000": 1.5773549999999998,
		"1e+06":  1.5773549999999998,
		"+Inf":   1.5773549999999998,
	}
	Convey("Counters comparison", t, func() {
		for _ = range expectTimes {
			labels, value := readCounter((<-ch).(prometheus.Metric))
			expect := fmt.Sprintf("[%s] %v", labels["le"], expectTimes[labels["le"]])
			got := fmt.Sprintf("[%s] %v", labels["le"], value)
			So(expect, ShouldEqual, got)
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
	Convey("Histogram comparison", t, func() {
		So(expectPb.Histogram, ShouldResemble, gotPb.Histogram)
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
	Convey("Various .my.cnf configurations", t, func() {
		Convey("Local tcp connection", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig))
			So(dsn, ShouldEqual, "root:abc123@tcp(localhost:3306)/")
		})
		Convey("Local tcp connection on non-default port", func() {
			dsn, _ := parseMycnf([]byte(tcpConfig2))
			So(dsn, ShouldEqual, "root:abc123@tcp(localhost:3308)/")
		})
		Convey("Socket connection", func() {
			dsn, _ := parseMycnf([]byte(socketConfig))
			So(dsn, ShouldEqual, "user:pass@unix(/var/lib/mysql/mysql.sock)/")
		})
		Convey("Socket connection ignoring defined host", func() {
			dsn, _ := parseMycnf([]byte(socketConfig2))
			So(dsn, ShouldEqual, "dude:nopassword@unix(/var/lib/mysql/mysql.sock)/")
		})
		Convey("Remote connection", func() {
			dsn, _ := parseMycnf([]byte(remoteConfig))
			So(dsn, ShouldEqual, "dude:nopassword@tcp(1.2.3.4:3307)/")
		})
		Convey("Missed user", func() {
			_, err := parseMycnf([]byte(badConfig))
			So(err, ShouldNotBeNil)
		})
		Convey("Missed password", func() {
			_, err := parseMycnf([]byte(badConfig2))
			So(err, ShouldNotBeNil)
		})
		Convey("No [client] section", func() {
			_, err := parseMycnf([]byte(badConfig3))
			So(err, ShouldNotBeNil)
		})
		Convey("Invalid config", func() {
			_, err := parseMycnf([]byte(badConfig4))
			So(err, ShouldNotBeNil)
		})
	})
}
