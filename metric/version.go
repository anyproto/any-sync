package metric

import (
	"github.com/anytypeio/any-sync/app"
	"github.com/prometheus/client_golang/prometheus"
	"runtime/debug"
)

func newVersionsCollector(a *app.App) prometheus.Collector {
	return &versionCollector{prometheus.MustNewConstMetric(prometheus.NewDesc(
		"anysync_versions",
		"Build information about the main Go module.",
		nil, prometheus.Labels{
			"anysync_version": anySyncVerion(),
			"app_name":        a.AppName(),
			"app_version":     a.Version(),
		},
	), prometheus.GaugeValue, 1)}
}

func anySyncVerion() string {
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, mod := range info.Deps {
			if mod.Path == "github.com/anytypeio/any-sync" {
				return mod.Version
			}
		}
	}
	return ""
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
