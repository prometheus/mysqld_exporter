local grafana = import 'github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';


  (import 'config.libsonnet') +
{


  grafanaDashboards::
    if $._config.enableLokiLogs then {
    'mysql-overview.json': (import 'dashboards/mysql-overview.json') +
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
      panels+: import 'lib/loki-panels.json'
    },
    }
    else {
    'mysql-overview.json': (import 'dashboards/mysql-overview.json'),
  },

  // Helper function to ensure that we don't override other rules, by forcing
  // the patching of the groups list, and not the overall rules object.
  local importRules(rules) = {
    groups+: std.native('parseYaml')(rules)[0].groups,
  },

  prometheusRules+: importRules(importstr 'rules/rules.yaml'),

  prometheusAlerts+:
    importRules(importstr 'alerts/general.yaml') +
    importRules(importstr 'alerts/galera.yaml'),
}