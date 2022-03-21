# MySQLd Mixin

The MySQLd Mixin is a set of configurable, reusable, and extensible alerts and
dashboards based on the metrics exported by the MySQLd Exporter and Loki logs (optional). The mixin also creates
recording and alerting rules for Prometheus and suitable dashboard descriptions
for Grafana.

MySQL Overview:
![screenshot-0](https://storage.googleapis.com/grafanalabs-integration-assets/mysql/screenshots/screenshot0.png)
MySQL Logs from Loki(optional):
![screenshot-1](https://storage.googleapis.com/grafanalabs-integration-assets/mysql/screenshots/screenshot1.png)

## Generate config files

You can manually generate dashboards, but first you should install some tools:

```bash
go install github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb@latest
go install github.com/google/go-jsonnet/cmd/jsonnet@latest
# or in brew: brew install go-jsonnet
```

For linting and formatting, you would also need `mixtool` and `jsonnetfmt` installed. If you
have a working Go development environment, it's easiest to run the following:

```bash
go install github.com/monitoring-mixins/mixtool/cmd/mixtool@latest
go install github.com/google/go-jsonnet/cmd/jsonnetfmt@latest
```

You can then build the Prometheus rules files `alerts.yaml` and
`rules.yaml` and a directory `dashboard_out` with the JSON dashboard files

for Grafana:
```bash
$ make build
```

### Loki Logs configuration

For proper logs correlation, you need to make sure that `job` and `instance` labels values match for both mysql_exporter metrics and logs, collected by promtail or grafana agent.

To enable logs support in MySQLd mixin, enable them in config.libsonnet first:

```
{
  _config+:: {
    enableLokiLogs: true,
  },
}

```

then run
```bash
$ make build
```

This would generate mysql logs dashboard, as well as modified mysql overview dashboard.

For more advanced uses of mixins, see
https://github.com/monitoring-mixins/docs.
