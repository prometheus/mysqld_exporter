package collector

import (
	"context"
	"database/sql"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/smartystreets/goconvey/convey"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

const dsn = "root@/mysql"

func TestExporter(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	exporter := New(
		context.Background(),
		db,
		NewMetrics(""),
		[]Scraper{
			ScrapeGlobalStatus{},
		},
	)

	convey.Convey("Metrics describing", t, func() {
		ch := make(chan *prometheus.Desc)
		go func() {
			exporter.Describe(ch)
			close(ch)
		}()

		for range ch {
		}
	})

	convey.Convey("Metrics collection", t, func() {
		ch := make(chan prometheus.Metric)
		go func() {
			exporter.Collect(ch)
			close(ch)
		}()

		for m := range ch {
			got := readMetric(m)
			if got.labels[model.MetricNameLabel] == "mysql_up" {
				convey.So(got.value, convey.ShouldEqual, 1)
			}
		}
	})
}

func TestGetMySQLVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error opening a stub database connection: %s", err)
	}
	defer db.Close()

	ctx := context.Background()
	convey.Convey("MySQL version extract", t, func() {
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow(""))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 999)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("something"))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 999)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("10.1.17-MariaDB"))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 10.1)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.7.13-6-log"))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 5.7)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.6.30-76.3-56-log"))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 5.6)
		mock.ExpectQuery(versionQuery).WillReturnRows(sqlmock.NewRows([]string{""}).AddRow("5.5.51-38.1"))
		convey.So(getMySQLVersion(ctx, db), convey.ShouldEqual, 5.5)
	})

	// Ensure all SQL queries were executed
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
