# Zen-Lead Deployment Files

This directory contains deployment manifests and monitoring configurations for zen-lead.

## Directory Structure

```
deploy/
├── prometheus/
│   └── prometheus-rules.yaml    # Prometheus alert rules
├── grafana/
│   └── dashboard.json           # Grafana dashboard
└── README.md                    # This file
```

## Prometheus Alert Rules

### Installation

**Option 1: Prometheus Operator (Recommended)**

```bash
kubectl apply -f deploy/prometheus/prometheus-rules.yaml
```

**Option 2: ConfigMap (for kube-prometheus-stack)**

```bash
kubectl create configmap zen-lead-alerts \
  --from-file=prometheus-rules.yaml=deploy/prometheus/prometheus-rules.yaml \
  -n monitoring

kubectl label configmap zen-lead-alerts \
  prometheus=kube-prometheus-stack \
  role=alert-rules \
  -n monitoring
```

### Alert Groups

- **zen-lead.critical**: Critical alerts requiring immediate attention
  - Controller down
  - No pods available
  - Leader service without endpoints
  - High failover rate
  - High reconciliation error rate

- **zen-lead.warning**: Warning alerts for investigation
  - Slow reconciliation
  - Port resolution failures
  - Low pod availability
  - High reconciliation rate
  - High sticky leader miss rate

- **zen-lead.info**: Informational alerts for tracking
  - Failover occurred
  - Port resolution failure

## Grafana Dashboard

### Installation

**Option 1: ConfigMap (for Grafana Operator)**

```bash
kubectl create configmap zen-lead-dashboard \
  --from-file=dashboard.json=deploy/grafana/dashboard.json \
  -n monitoring

kubectl label configmap zen-lead-dashboard \
  grafana_dashboard=1 \
  -n monitoring
```

**Option 2: Manual Import**

1. Open Grafana UI
2. Go to Dashboards → Import
3. Upload `deploy/grafana/dashboard.json`
4. Select your Prometheus data source
5. Click Import

### Dashboard Panels

The dashboard includes:

1. **Overview Stats**
   - Leader Services Total
   - Pods Available
   - Failover Count (Total)
   - Failover Rate

2. **Leader Metrics**
   - Leader Duration by Service
   - Leader Pod Age
   - Leader Selection Attempts

3. **Availability Metrics**
   - Pods Available by Service
   - Leader Services Without Endpoints

4. **Performance Metrics**
   - Reconciliation Duration (P95, P50)
   - Reconciliation Rate
   - Reconciliation Errors

5. **Stability Metrics**
   - Failover Rate Over Time
   - Sticky Leader Hit/Miss Rate

6. **Configuration Metrics**
   - Port Resolution Failures

7. **Resource Metrics**
   - Leader Services and EndpointSlices by Namespace (Table)

## Metrics Reference

All metrics are prefixed with `zen_lead_`:

- `zen_lead_leader_duration_seconds` - How long a pod has been leader
- `zen_lead_failover_count_total` - Total failovers
- `zen_lead_reconciliation_duration_seconds` - Reconciliation duration histogram
- `zen_lead_pods_available` - Ready pods count
- `zen_lead_port_resolution_failures_total` - Port resolution failures
- `zen_lead_reconciliation_errors_total` - Reconciliation errors
- `zen_lead_leader_services_total` - Leader services count
- `zen_lead_endpointslices_total` - EndpointSlices count
- `zen_lead_sticky_leader_hits_total` - Sticky leader hits
- `zen_lead_sticky_leader_misses_total` - Sticky leader misses
- `zen_lead_leader_selection_attempts_total` - Leader selection attempts
- `zen_lead_leader_pod_age_seconds` - Leader pod age
- `zen_lead_leader_service_without_endpoints` - Services without endpoints
- `zen_lead_reconciliations_total` - Total reconciliations

## Troubleshooting

### Alerts Not Firing

1. Verify Prometheus is scraping zen-lead metrics:
   ```bash
   kubectl port-forward -n zen-lead-system svc/zen-lead-metrics 8080:8080
   curl http://localhost:8080/metrics | grep zen_lead
   ```

2. Check PrometheusRule is loaded:
   ```bash
   kubectl get prometheusrule -n zen-lead-system
   ```

3. Verify alert rule expressions in Prometheus UI

### Dashboard Not Showing Data

1. Verify Prometheus data source is configured correctly
2. Check time range (default: last 1 hour)
3. Verify metrics are being scraped:
   ```bash
   curl http://prometheus:9090/api/v1/query?query=zen_lead_pods_available
   ```

## Customization

### Alert Thresholds

Edit `deploy/prometheus/prometheus-rules.yaml` to adjust:
- Alert thresholds (e.g., failover rate > 0.1)
- Alert durations (e.g., `for: 5m`)
- Alert severities

### Dashboard Panels

Edit `deploy/grafana/dashboard.json` to:
- Add/remove panels
- Change panel sizes/positions
- Modify queries
- Adjust refresh intervals

## Support

For issues or questions:
- GitHub Issues: https://github.com/kube-zen/zen-lead/issues
- Documentation: https://github.com/kube-zen/zen-lead/docs

