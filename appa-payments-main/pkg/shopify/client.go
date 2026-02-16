package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// GraphQLClient is a client for interacting with the Shopify GraphQL API
type GraphQLClient struct {
	endpoint string
	token    string
	client   *http.Client
	logger   *zap.Logger
}

// NewGraphQLClient creates a new Shopify API client
func NewGraphQLClient(
	shopDomain,
	apiVersion,
	adminToken string,
	logger *zap.Logger,
) *GraphQLClient {

	return &GraphQLClient{
		endpoint: fmt.Sprintf("https://%s/admin/api/%s/graphql.json", shopDomain, apiVersion),
		token:    adminToken,
		client:   &http.Client{Timeout: 20 * time.Second},
		logger:   logger,
	}
}

// Do executes a GraphQL request
func (g *GraphQLClient) Do(
	ctx context.Context,
	query string,
	vars map[string]any,
	out any,
) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		g.logger.Error(err.Error())
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, g.endpoint, bytes.NewReader(body),
	)
	if err != nil {
		g.logger.Error(err.Error())
		return err
	}

	// g.logger.Info("Shopify GraphQL Request", zap.String("query", query), zap.Any("variables", vars))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		g.logger.Error(err.Error())
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.Error(err.Error())
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		g.logger.Error(fmt.Sprintf("shopify graphql http %d: %s", resp.StatusCode, string(data)))
		return fmt.Errorf("shopify graphql http %d: %s", resp.StatusCode, string(data))
	}

	var envelope gqlResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		g.logger.Error(err.Error())
		return err
	}

	if len(envelope.Errors) > 0 {
		return fmt.Errorf("shopify graphql errors: %+v", envelope.Errors)
	}

	if out != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, out)
	}

	return nil
}

// GID generates a Shopify GraphQL global ID
func GID(kind string, id string) string {
	return fmt.Sprintf("gid://shopify/%s/%s", kind, id)
}
