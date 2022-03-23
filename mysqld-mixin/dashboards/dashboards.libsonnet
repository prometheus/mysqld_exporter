local grafana = import 'github.com/grafana/grafonnet-lib/grafonnet/grafana.libsonnet';

{

  grafanaDashboards::
    if $._config.enableLokiLogs then {
      'mysql-overview.json':
        (import 'mysql-overview.json')
        +
        {
          links+: [
            {
              asDropdown: false,
              icon: 'dashboard',
              includeVars: true,
              keepTime: true,
              tags: [],
              targetBlank: false,
              title: 'MySQL Logs',
              tooltip: '',
              type: 'link',
              url: 'd/DlHAFwE7z',
            },
          ],
        },
      'mysql-logs.json': (import 'mysql-logs.json'),
    }
    else {
      'mysql-overview.json': (import 'mysql-overview.json'),
    },
}
