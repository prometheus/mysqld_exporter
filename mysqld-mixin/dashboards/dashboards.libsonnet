local grafana = import 'github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';

{

  grafanaDashboards::
    if $._config.enableLokiLogs then {
    'mysql-overview.json': (import 'mysql-overview.json') +
    {
      templating+:
        {
          list+: [
            grafana.template.new(
            name="loki_datasource",
            label="Logs Data Source",
            query='loki',
            refresh='load',
            datasource='loki'
            ) + { type: "datasource"}
          ]
        },
      panels+: import '../lib/loki-panels.json'
    },
    }
    else {
    'mysql-overview.json': (import 'mysql-overview.json'),
  }
}