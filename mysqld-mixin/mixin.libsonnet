{
  grafanaDashboards+: {
    'mysql-overview.json': (import 'dashboards/mysql-overview.json'),
  },

  // Helper function to ensure that we don't override other rules, by forcing
  // the patching of the groups list, and not the overall rules object.
  local importRules(rules) = {
    groups+: std.parseYaml(rules).groups,
  },

  prometheusRules+: importRules(importstr 'rules/rules.yaml'),

  prometheusAlerts+:
    importRules(importstr 'alerts/general.yaml') +
    importRules(importstr 'alerts/galera.yaml'),
}
