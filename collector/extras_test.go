// Copyright 2024 PlanetScale, Inc. to appease `make check_license`
package collector

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/smartystreets/goconvey/convey"
)

const (
	testFooBar       = "foobar"
	testFooBarQuery  = "SELECT 47 AS foo, 48879 AS bar;"
	testBazQuux      = "bazquux"
	testBazQuuxQuery = "SELECT 3.14159 AS baz, 2.71828 AS quux;"
)

func TestExtrasInit(t *testing.T) {
	f := createTemp(t)
	defer removeTemp(t, f)
	writeExtrasYAML(t, f)                  // write first
	e, err := newExtras(f.Name(), "", nil) // then read during initialization
	if err != nil {
		t.Fatal(err)
	}
	testExtraScrapers(t, e)
}

func TestExtrasRefresh(t *testing.T) {
	f := createTemp(t)
	defer removeTemp(t, f)
	e, err := newExtras(f.Name(), "500ms", nil) // initialize empty
	if err != nil {
		t.Fatal(err)
	}
	if scrapers := e.Scrapers(); len(scrapers) != 0 { // test that the list is empty
		t.Fatal(scrapers)
	}
	writeExtrasYAML(t, f)   // then write
	time.Sleep(time.Second) // could e.Refresh() but this exercises the goroutine, too
	testExtraScrapers(t, e)
}

func createTemp(t *testing.T) *os.File {
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func removeTemp(t *testing.T, f *os.File) {
	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func testExtraScrapers(t *testing.T, e *Extras) {
	scrapers := e.Scrapers()
	if len(scrapers) != 2 {
		t.Fatal(scrapers)
	}
	if scrapers[0].Metric != testFooBar || scrapers[0].Query != testFooBarQuery {
		t.Fatal(scrapers)
	}
	if scrapers[1].Metric != testBazQuux || scrapers[1].Query != testBazQuuxQuery {
		t.Fatal(scrapers)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(testFooBarQuery).WillReturnRows(sqlmock.NewRows([]string{"foo", "bar"}).AddRow(47, 48879))
	mock.ExpectQuery(testBazQuuxQuery).WillReturnRows(sqlmock.NewRows([]string{"baz", "quux"}).AddRow(3.14159, 2.71828))
	ch := make(chan prometheus.Metric)
	go func() {
		if err = e.Scrape(context.Background(), db, ch, log.NewNopLogger()); err != nil {
			t.Error(err)
		}
		close(ch)
	}()
	expected := []MetricResult{
		{labels: labelMap{"column": "foo"}, value: 47, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"column": "bar"}, value: 48879, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"column": "baz"}, value: 3.14159, metricType: dto.MetricType_GAUGE},
		{labels: labelMap{"column": "quux"}, value: 2.71828, metricType: dto.MetricType_GAUGE},
	}
	convey.Convey("Metrics comparison", t, func() {
		for _, expect := range expected {
			got := readMetric(<-ch)
			convey.So(expect, convey.ShouldResemble, got)
		}
	})
	if m := <-ch; m != nil {
		t.Fatalf("unexpected %+v", readMetric(m))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func writeExtrasYAML(t *testing.T, f *os.File) {
	if _, err := f.Write([]byte(fmt.Sprintf(`---
- metric: %s
  query: %s
- metric: %s
  query: %s
...
`, testFooBar, testFooBarQuery, testBazQuux, testBazQuuxQuery))); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
}
