// Package nria provides an otelcol.receiver.nria component.
package nria

import (
	"slices"
	"time"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/nria/nriareceiver"
	"github.com/grafana/alloy/internal/featuregate"
)

// TODO: handle "connect" step and use it to populate entity data
// TODO: cache metrics/resources/etc
// TODO: generate logs from System events (user login, etc...)
// TODO: generate logs from Inventory events (a new package is installed, etc...)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.nria",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := nriareceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.nria component.
type Arguments struct {
	HTTPServer otelcol.HTTPServerArguments `alloy:",squash"`

	ReadTimeout time.Duration `alloy:"read_timeout,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		HTTPServer: otelcol.HTTPServerArguments{
			Endpoint:              ":4444",
			CompressionAlgorithms: slices.Clone(otelcol.DefaultCompressionAlgorithms),
		},
		ReadTimeout: 60 * time.Second,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	convertedHttpServer, err := args.HTTPServer.Convert()
	if err != nil {
		return nil, err
	}

	return &nriareceiver.Config{
		ServerConfig: *convertedHttpServer,
		ReadTimeout:  args.ReadTimeout,
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.HTTPServer.Extensions()
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
