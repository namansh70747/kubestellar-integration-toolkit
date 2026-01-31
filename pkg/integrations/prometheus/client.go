package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Client struct {
	api        promv1.API
	httpClient *http.Client
	baseURL    string
}

type QueryResult struct {
	Metric map[string]string
	Values []SamplePair
}

type SamplePair struct {
	Timestamp time.Time
	Value     float64
}

func NewClient(prometheusURL string) (*Client, error) {
	cfg := api.Config{
		Address: prometheusURL,
	}

	apiClient, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &Client{
		api:        promv1.NewAPI(apiClient),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    prometheusURL,
	}, nil
}

func (c *Client) Query(ctx context.Context, query string, ts time.Time) ([]QueryResult, error) {
	result, warnings, err := c.api.Query(ctx, query, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(warnings) > 0 {
		fmt.Printf("Prometheus warnings: %v\n", warnings)
	}

	return c.parseQueryResult(result)
}

func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]QueryResult, error) {
	r := promv1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	result, warnings, err := c.api.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("failed to execute range query: %w", err)
	}

	if len(warnings) > 0 {
		fmt.Printf("Prometheus warnings: %v\n", warnings)
	}

	return c.parseQueryResult(result)
}

func (c *Client) parseQueryResult(result model.Value) ([]QueryResult, error) {
	var queryResults []QueryResult

	switch v := result.(type) {
	case model.Vector:
		for _, sample := range v {
			metric := make(map[string]string)
			for k, v := range sample.Metric {
				metric[string(k)] = string(v)
			}
			queryResults = append(queryResults, QueryResult{
				Metric: metric,
				Values: []SamplePair{
					{
						Timestamp: sample.Timestamp.Time(),
						Value:     float64(sample.Value),
					},
				},
			})
		}
	case model.Matrix:
		for _, stream := range v {
			metric := make(map[string]string)
			for k, v := range stream.Metric {
				metric[string(k)] = string(v)
			}
			var values []SamplePair
			for _, sp := range stream.Values {
				values = append(values, SamplePair{
					Timestamp: sp.Timestamp.Time(),
					Value:     float64(sp.Value),
				})
			}
			queryResults = append(queryResults, QueryResult{
				Metric: metric,
				Values: values,
			})
		}
	default:
		return nil, fmt.Errorf("unsupported result type: %T", result)
	}

	return queryResults, nil
}

func (c *Client) ValidateConnection(ctx context.Context) error {
	_, err := c.api.Runtimeinfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Prometheus: %w", err)
	}
	return nil
}

func (c *Client) GetTargets(ctx context.Context) (promv1.TargetsResult, error) {
	result, err := c.api.Targets(ctx)
	if err != nil {
		return promv1.TargetsResult{}, fmt.Errorf("failed to get targets: %w", err)
	}
	return result, nil
}

func (c *Client) GetAlerts(ctx context.Context) (promv1.AlertsResult, error) {
	result, err := c.api.Alerts(ctx)
	if err != nil {
		return promv1.AlertsResult{}, fmt.Errorf("failed to get alerts: %w", err)
	}
	return result, nil
}

func (c *Client) GetMetrics(ctx context.Context) ([]string, error) {
	result, warnings, err := c.api.LabelValues(ctx, "__name__", nil, time.Now().Add(-1*time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	if len(warnings) > 0 {
		fmt.Printf("Prometheus warnings: %v\n", warnings)
	}

	var metrics []string
	for _, m := range result {
		metrics = append(metrics, string(m))
	}

	return metrics, nil
}
