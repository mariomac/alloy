package metadata

import (
	"go.opentelemetry.io/collector/component"

	"github.com/grafana/alloy/internal/featuregate"
)
var (
	Type      = component.MustNewType("nria")
	ScopeName = "otel_scope_name"
)

const (
	TracesStability  = featuregate.StabilityExperimental
	MetricsStability = featuregate.StabilityExperimental
)
