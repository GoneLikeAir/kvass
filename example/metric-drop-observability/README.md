# kvass metric-drop fail-open observability example

These files show how to scrape the sidecar's dedicated fail-open counters and
evaluate increment-based alerts. They are not applied by building kvass.
Collection, rules, and routing must be wired in a separate deployment change.

## What to scrape

Sidecar API listen address (`--web.api-addr`) defaults to **port 8080**.

| Path | Owner | Use |
| --- | --- | --- |
| `GET /api/v1/shard/metrics/` | kvass sidecar | fail-open counters |
| `GET /metrics` | reverse-proxy to embedded Prometheus | **do not** replace this with the kvass counter job |

Counter:

```
kvass_metric_drop_fail_open_total{reason,filtering_active}
```

`reason` is one of `unexpected_response`, `unsupported_format`, `parse_error`,
`internal_panic`. `filtering_active` is `true` or `false`. There are no job,
URL, target, generation, or error-text labels.

The same process cumulative counts are also on `GET /api/v1/shard/runtimeinfo/`
(`dropSetFailOpen`, `dropSetFailOpenByReason`, `dropSetLastFailure`).
`dropSetLastFailure` is the last event, not a "still failing" flag.

Target attribution: sidecar logs include `job`, existing target hash
(`targetId`), and scheme/host/port. Compare those with Prometheus
`/api/v1/targets` around the event time. Sidecar HTTP success only means the
upstream scrape and forward succeeded, not that Prometheus ingested a series.

## Static target

See [prometheus-scrape.yml](prometheus-scrape.yml):

```yaml
scrape_configs:
  - job_name: kvass-sidecar-fail-open
    metrics_path: /api/v1/shard/metrics/
    scrape_interval: 15s
    static_configs:
      - targets:
          - 127.0.0.1:8080
```

A commented Kubernetes pod SD fragment is in the same file. Point `__address__`
at the sidecar API port 8080, not the Prometheus scrape proxy port.

## Alerting

See [rules.yml](rules.yml). Alerts use `increase(...[5m])` on
`filtering_active="true"` plus a pending window. Do not alert on a lifetime
total greater than zero.

`KvassSidecarFailOpenMetricsAbsent` covers scrape gaps (`up == 0` or absent).
If fail-open alerts go inactive while that availability alert fires, treat it
as missing data, not recovery.

These rules do **not** claim metric leakage. `up=1` only means this metrics
path was scraped. They do not estimate how many series were forwarded unfiltered.

Boundary cases are encoded in [rules.test.yml](rules.test.yml): trigger, stop
and recover, lifetime total stays >0, counter reset, new pod, old pod, scrape
missing, internal panic, filtering inactive.

Check locally:

```bash
promtool check rules example/metric-drop-observability/rules.yml
promtool test rules example/metric-drop-observability/rules.test.yml
```

## Independent configuration follow-ups (not done here)

These remain separate from this code change and are not executed by it:

1. Persist the `knative` job so it keeps `http-usermetric` without dropping
   Knative 9090 service capability.
2. Find the real source of `1643-vmu/cccs-rsa` and `1643-31w/cccs-rsa` and add
   `prometheus.io/path=/metrics/prometheus` there, instead of only patching
   generated objects.

## Bounded logs

One log combines the current event with `suppressed`, the number of pending
repeats for that job/target/reason. The fixed cache holds 256 keys and evicts in
FIFO order. `evictedSuppressed` is a process-wide aggregate from evicted keys,
not a count for the target named in that log. Both aggregates are retained until
an event can be logged. Each sidecar emits at most 10 fail-open logs per second,
with at most one per key per minute; summaries share the same log and budget.
Counters continue to count every event independently of logging. Pending logs
are emitted on subsequent events, not by a background flush timer.
