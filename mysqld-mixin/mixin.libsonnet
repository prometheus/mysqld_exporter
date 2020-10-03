{
    grafanaDashboards: {
        "mysql-overview.json": (import "dashboards/mysql-overview.json"),
    },

    prometheusRules: std.native('parseYaml')(importstr 'rules/rules.yaml'),

    prometheusAlerts:  std.native('parseYaml')(importstr 'alerts/galera.yaml'),
}
