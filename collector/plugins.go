package collector

import (
	"context"
	"database/sql"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	pluginsQuery = `SHOW PLUGINS`
)

var (
	pluginDescription = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "plugin"),
		"MySQL plugin",
		[]string{"name", "status", "type", "library", "license"}, nil,
	)
)

type plugin struct {
	Name    sql.NullString
	Status  sql.NullString
	Type    sql.NullString
	Library sql.NullString
	License sql.NullString
}

// ScrapePlugins collects from `SHOW PLUGINS`.
type ScrapePlugins struct{}

// Name of the Scraper. Should be unique.
func (ScrapePlugins) Name() string {
	return "plugins"
}

// Help describes the role of the Scraper.
func (ScrapePlugins) Help() string {
	return "Collect from SHOW PLUGINS"
}

// Version of MySQL from which scraper is available.
func (ScrapePlugins) Version() float64 {
	return 5.1
}

func (ScrapePlugins) Scrape(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, logger log.Logger) error {
	showPluginsRows, err := db.QueryContext(ctx, pluginsQuery)
	if err != nil {
		return err
	}
	defer showPluginsRows.Close()
	for showPluginsRows.Next() {
		var pluginVal plugin
		if err := showPluginsRows.Scan(&pluginVal.Name, &pluginVal.Status, &pluginVal.Type, &pluginVal.Library, &pluginVal.License); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			pluginDescription, prometheus.GaugeValue, 1,
			pluginVal.Name.String, pluginVal.Status.String, pluginVal.Type.String, pluginVal.Library.String, pluginVal.License.String,
		)
	}

	return nil
}

// check interface
var _ Scraper = ScrapePlugins{}
