package http_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// findOpenAPISpec locates the openapi.yaml file by walking up from the test directory.
func findOpenAPISpec(t *testing.T) string {
	// Start from the current working directory or test file location
	dir, _ := os.Getwd()

	// Look for api/openapi.yaml by going up directories
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "api", "openapi.yaml")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}

	t.Fatalf("could not find api/openapi.yaml")
	return ""
}

// TestOpenAPISpec validates the OpenAPI specification is valid.
func TestOpenAPISpec(t *testing.T) {
	// Load the spec file
	specPath := findOpenAPISpec(t)
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}

	// Parse YAML spec
	loader := &openapi3.Loader{IsExternalRefsAllowed: false}
	spec, err := loader.LoadFromData(data)
	if err != nil {
		t.Fatalf("failed to parse OpenAPI spec: %v", err)
	}

	// Validate the spec
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI spec validation failed: %v", err)
	}

	// Check that key paths exist
	expectedPaths := []string{
		"/v1/health",
		"/v1/ready",
		"/v1/agencies",
		"/v1/agencies/{slug}",
		"/v1/agencies/{slug}/routes",
		"/v1/agencies/{slug}/stats", // NEW
		"/v1/stops/nearby",
		"/v1/stops/search",
		"/v1/stops/batch",
		"/v1/stops/{id}",
		"/v1/stops/{id}/departures",
		"/v1/stops/{id}/routes",
		"/v1/routes",
		"/v1/routes/{id}",
		"/v1/routes/{id}/vehicles",
		"/v1/routes/{id}/stops", // NEW
		"/v1/trips/{id}",
		"/v1/trips/{id}/stop-times",
		"/v1/feeds/status",
		"/v1/journeys", // NEW
		"/graphql",
	}

	for _, path := range expectedPaths {
		if item := spec.Paths.Find(path); item == nil {
			t.Errorf("expected path %s not found in spec", path)
		}
	}

	// Verify key schemas exist
	expectedSchemas := []string{
		"Agency",
		"Stop",
		"Route",
		"Trip",
		"StopTime",
		"VehiclePosition",
		"Departure",
		"FeedStats",
		"APIError",
		"Pagination",
	}

	for _, schema := range expectedSchemas {
		if spec.Components.Schemas[schema] == nil {
			t.Errorf("expected schema %s not found", schema)
		}
	}

	t.Logf("OpenAPI spec valid: %d paths, %d schemas", len(spec.Paths.Map()), len(spec.Components.Schemas))
}

// TestOpenAPIInfo verifies spec metadata.
func TestOpenAPIInfo(t *testing.T) {
	specPath := findOpenAPISpec(t)
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}

	loader := &openapi3.Loader{IsExternalRefsAllowed: false}
	spec, err := loader.LoadFromData(data)
	if err != nil {
		t.Fatalf("failed to parse OpenAPI spec: %v", err)
	}

	if spec.Info.Title != "BilboPass Transit API" {
		t.Errorf("expected title 'BilboPass Transit API', got %q", spec.Info.Title)
	}

	if spec.Info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %q", spec.Info.Version)
	}

	if spec.Info.Description == "" {
		t.Error("expected non-empty description")
	}

	if len(spec.Servers) == 0 {
		t.Error("expected at least one server")
	}

	t.Logf("OpenAPI Info: %s v%s @ %s", spec.Info.Title, spec.Info.Version, spec.Servers[0].URL)
}
