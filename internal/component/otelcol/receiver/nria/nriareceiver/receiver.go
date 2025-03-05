package nriareceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/nria/nriareceiver/internal/nria"
)

type nriaReceiver struct {
	anress string
	config  *Config
	params  receiver.Settings

	nextTracesConsumer  consumer.Traces
	nextMetricsConsumer consumer.Metrics

	server    *http.Server
	tReceiver *receiverhelper.ObsReport
}

// Endpoint specifies an API endpoint definition.
type Endpoint struct {
	// Pattern specifies the API pattern, as registered by the HTTP handler.
	Pattern string

	// Handler specifies the http.Handler for this endpoint.
	Handler func(http.ResponseWriter, *http.Request)
}

// getEndpoints specifies the list of endpoints registered for the trace-agent API.
func (nr *nriaReceiver) getEndpoints() []Endpoint {
	endpoints := []Endpoint{
		{
			Pattern: "/",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	// connect stuff
	endpoints = append(endpoints, Endpoint{
		Pattern: "/identity/v1/connect",
		Handler: nr.handleConnect,
	})
	if nr.nextTracesConsumer != nil {
		// TODO: manage traces here
	}

	if nr.nextMetricsConsumer != nil {
		endpoints = append(endpoints, []Endpoint{{
			Pattern: "/infra/v2/metrics/events/bulk",
			Handler: nr.handleBulkEvents,
		}, {
			Pattern: "/metrics/events/bulk",
			Handler: nr.handleBulkEvents,
		}}...)
	}
	return endpoints
}

func newNRIAReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &nriaReceiver{
		params: params,
		config: config,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver: instance,
	}, nil
}

func (nr *nriaReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	endpoints := nr.getEndpoints()

	for _, e := range endpoints {
		ddmux.HandleFunc(e.Pattern, e.Handler)
	}

	var err error
	nr.server, err = nr.config.ServerConfig.ToServer(
		ctx,
		host,
		nr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := nr.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	nr.anress = hln.Addr().String()

	go func() {
		if err := nr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
		}
	}()
	return nil
}

func (nr *nriaReceiver) Shutdown(ctx context.Context) (err error) {
	return nr.server.Shutdown(ctx)
}

// handleInfo handles incoming /info payloads.
func (nr *nriaReceiver) handleInfo(w http.ResponseWriter, _ *http.Request, infoResponse []byte) {
	_, err := fmt.Fprintf(w, "%s", infoResponse)
	if err != nil {
		nr.params.Logger.Error("Error writing /info endpoint response", zap.Error(err))
		http.Error(w, "Error writing /info endpoint response", http.StatusInternalServerError)
		return
	}
}

func (fc *nriaReceiver) handleConnect(writer http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logrus.WithError(err).Error("Reading request body")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := struct {
		Fingerprint nria.Fingerprint `json:"fingerprint"`
		Type        string      `json:"type"`
		Protocol    string      `json:"protocol"`
		EntityID    nria.EntityID    `json:"entityId,omitempty"`
	}{}

	if err := json.Unmarshal(body, &req); err != nil {
		logrus.WithError(err).Error("parsing request body mapeao")
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if request.Method == http.MethodPost {
		logrus.WithField("connect", req).Info("received connect fingerprint")
	} else if request.Method == http.MethodPut && req.EntityID > 0 {
		logrus.WithField("reconnect", req).Info("received update fingerprint")
	} else {
		writer.WriteHeader(http.StatusBadRequest)
	}

	writer.WriteHeader(http.StatusOK)
}

func (nr *nriaReceiver) handleBulkEvents(w http.ResponseWriter, req *http.Request) {
	// use datapoint.convert to get data
	// then forward it using any of the examples below
}

// handleV2Series handles the v2 series endpoint https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func (nr *nriaReceiver) handleV2Series(w http.ResponseWriter, req *http.Request) {
	obsCtx := nr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		nr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	series, err := nr.metricsTranslator.HandleSeriesV2Payload(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		nr.params.Logger.Error(err.Error())
		return
	}

	metrics := nr.metricsTranslator.TranslateSeriesV2(series)
	metricsCount = metrics.DataPointCount()

	err = nr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		errorutil.HTTPError(w, err)
		nr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]any{
		"errors": []string{},
	}
	_ = json.NewEncoder(w).Encode(response)
}
