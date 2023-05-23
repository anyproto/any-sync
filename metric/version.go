package metric

import (
	"github.com/anyproto/any-sync/app"
	"github.com/prometheus/client_golang/prometheus"
)

func newVersionsCollector(a *app.App) prometheus.Collector {
	return &versionCollector{prometheus.MustNewConstMetric(prometheus.NewDesc(
		"anysync_versions",
		"Build information about the main Go module.",
		nil, prometheus.Labels{
			"anysync_version": a.AnySyncVersion(),
			"app_name":        a.AppName(),
			"app_version":     a.Version(),
		},
	), prometheus.GaugeValue, 1)}
}

type versionCollector struct {
	ver prometheus.Metric
}

func (v *versionCollector) Describe(descs chan<- *prometheus.Desc) {
	descs <- v.ver.Desc()
}

func (v *versionCollector) Collect(metrics chan<- prometheus.Metric) {
	metrics <- v.ver
}
