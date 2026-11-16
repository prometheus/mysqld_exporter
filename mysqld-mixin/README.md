# MySQLd Mixin

The MySQLd Mixin is a set of configurable, reusable, and extensible alerts and
dashboards based on the metrics exported by the MySQLd Exporter. The mixin creates
recording and alerting rules for Prometheus and suitable dashboard descriptions
for Grafana.

To use them, you need to have `mixtool` and `jsonnetfmt` installed. If you
have a working Go development environment, it's easiest to run the following:
```bash
$ go install github.com/monitoring-mixins/mixtool/cmd/mixtool@4e5c5015347639a73771cac72e74429847d2710e
$ go install github.com/google/go-jsonnet/cmd/jsonnetfmt@latest
```

Mixtool is temporarily pinned because newer versions fail to install with
`go install`. Remove the pin once [the upstream issue](https://github.com/monitoring-mixins/mixtool/issues/261)
is fixed.

You can then build the Prometheus rules files `alerts.yaml` and
`rules.yaml` and a directory `dashboard_out` with the JSON dashboard files
for Grafana:
```bash
$ make build
```

For more advanced uses of mixins, see
https://github.com/monitoring-mixins/docs.
