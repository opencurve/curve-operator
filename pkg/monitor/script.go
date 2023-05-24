package monitor

var PROMETHEUS_YML = `
global:
  scrape_interval: 3s
  evaluation_interval: 15s
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
    - targets: ['localhost:%d']
  - job_name: 'curve_metrics'
    file_sd_configs:
    - files: ['target.json']
  - job_name: 'node'
    static_configs:
      - targets: %s
`

var GRAFANA_DATA_SOURCE = `
datasources:
- name: 'Prometheus'
  type: 'prometheus'
  access: 'proxy'
  org_id: 1
  url: 'http://%s:%d'
  is_default: true
  version: 1
  editable: true
`
