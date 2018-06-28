package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/smartystreets/goconvey/convey"
)

const dsn = "root@/mysql"

func TestExporter(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping test")
	}

	exporter := New(
		dsn,
		NewMetrics(),
		[]Scraper{
			ScrapeGlobalStatus{},
		})

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
