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

## Loki Logs configuration

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

This would generate MySQL logs dashboard, as well as modified MySQL overview dashboard.

For proper logs correlation, you need to make sure that `job` and `instance` labels values match for both mysql_exporter metrics and logs, collected by [Promtail](https://grafana.com/docs/loki/latest/clients/promtail/) or [Grafana Agent](https://grafana.com/docs/grafana-cloud/agent/).

To scrape MySQL logs the following promtail config snippet can be used for `job=integrations/mysql` and `instance=mysql-01`:

```yaml
scrape_configs:
- job_name: integrations/mysql 
  static_configs:
    - labels:
        instance: mysql-01 # must match instance used in mysqld_exporter
        job: integrations/mysql # must match job used in mysqld_exporter
        __path__: /var/log/mysql/*.log
  pipeline_stages:
    - 
      # logs of mysql in sample-apps https://dev.mysql.com/doc/refman/8.0/en/error-log-format.html
      # format time thread [label] [err_code] [subsystem] msg
      # The [err_code] and [subsystem] fields were added in MySQL 8.0
      # https://regex101.com/r/jwEke3/2
      regex:
          expression: '(?P<timestamp>.+) (?P<thread>[\d]+) \[(?P<label>.+?)\]( \[(?P<err_code>.+?)\] \[(?P<subsystem>.+?)\])? (?P<msg>.+)'
    - labels:
        label:
        err_code:
        subsystem:
      # (optional) uncomment parse timestamp, but make sure you set the proper location to parse timezone
      #- timestamp:
      #    source: timestamp
      #    fallback_formats: ["2006-01-02 15:04:05"]
      #    format: "2006-01-02T15:04:05.000000Z"
      #    location: Etc/UTC
    - drop:
        expression: "^ *$"
        drop_counter_reason: "drop empty lines"
```
