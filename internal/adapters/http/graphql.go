package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/graphql-go/graphql"

	"github.com/samirrijal/bilbopass/internal/core/domain"
)

// buildSchema creates the GraphQL schema wired to our services.
func buildSchema(deps *Dependencies) (graphql.Schema, error) {
	geoPointType := graphql.NewObject(graphql.ObjectConfig{
		Name: "GeoPoint",
		Fields: graphql.Fields{
			"lat": &graphql.Field{Type: graphql.Float},
			"lon": &graphql.Field{Type: graphql.Float},
		},
	})

	agencyType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Agency",
		Fields: graphql.Fields{
			"id":       &graphql.Field{Type: graphql.String},
			"slug":     &graphql.Field{Type: graphql.String},
			"name":     &graphql.Field{Type: graphql.String},
			"url":      &graphql.Field{Type: graphql.String},
			"timezone": &graphql.Field{Type: graphql.String},
		},
	})

	routeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Route",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.String},
			"route_id":   &graphql.Field{Type: graphql.String},
			"agency_id":  &graphql.Field{Type: graphql.String},
			"short_name": &graphql.Field{Type: graphql.String},
			"long_name":  &graphql.Field{Type: graphql.String},
			"route_type": &graphql.Field{Type: graphql.Int},
			"color":      &graphql.Field{Type: graphql.String},
			"text_color": &graphql.Field{Type: graphql.String},
		},
	})

	vehicleType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Vehicle",
		Fields: graphql.Fields{
			"vehicle_id": &graphql.Field{Type: graphql.String},
			"trip_id":    &graphql.Field{Type: graphql.String},
			"route_id":   &graphql.Field{Type: graphql.String},
			"location":   &graphql.Field{Type: geoPointType},
			"bearing":    &graphql.Field{Type: graphql.Float},
			"speed":      &graphql.Field{Type: graphql.Float},
		},
	})

	stopType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Stop",
		Fields: graphql.Fields{
			"id":                    &graphql.Field{Type: graphql.String},
			"stop_id":               &graphql.Field{Type: graphql.String},
			"agency_id":             &graphql.Field{Type: graphql.String},
			"name":                  &graphql.Field{Type: graphql.String},
			"location":              &graphql.Field{Type: geoPointType},
			"platform_code":         &graphql.Field{Type: graphql.String},
			"wheelchair_accessible": &graphql.Field{Type: graphql.Boolean},
			"distance":              &graphql.Field{Type: graphql.Float},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"agencies": &graphql.Field{
				Type:        graphql.NewList(agencyType),
				Description: "List all transit agencies",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return deps.Agencies.List(p.Context)
				},
			},
			"stopsNearby": &graphql.Field{
				Type:        graphql.NewList(stopType),
				Description: "Find stops near a location",
				Args: graphql.FieldConfigArgument{
					"lat":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Float)},
					"lon":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Float)},
					"radius": &graphql.ArgumentConfig{Type: graphql.Float, DefaultValue: 500.0},
					"limit":  &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 20},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					lat := p.Args["lat"].(float64)
					lon := p.Args["lon"].(float64)
					radius := p.Args["radius"].(float64)
					limit := p.Args["limit"].(int)
					return deps.Stops.FindNearby(p.Context, lat, lon, radius, limit)
				},
			},
			"searchStops": &graphql.Field{
				Type:        graphql.NewList(stopType),
				Description: "Search stops by name (fuzzy matching)",
				Args: graphql.FieldConfigArgument{
					"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"limit": &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 20},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					q := p.Args["query"].(string)
					limit := p.Args["limit"].(int)
					return deps.Stops.Search(p.Context, q, nil, limit)
				},
			},
			"stop": &graphql.Field{
				Type:        stopType,
				Description: "Get a stop by ID",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id := p.Args["id"].(string)
					return deps.Stops.GetByID(p.Context, id)
				},
			},
			"route": &graphql.Field{
				Type:        routeType,
				Description: "Get a route by ID",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id := p.Args["id"].(string)
					return deps.Routes.GetByID(p.Context, id)
				},
			},
			"routesByAgency": &graphql.Field{
				Type:        graphql.NewList(routeType),
				Description: "List routes for an agency",
				Args: graphql.FieldConfigArgument{
					"agency_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					agencyID := p.Args["agency_id"].(string)
					return deps.Routes.ListByAgency(p.Context, agencyID)
				},
			},
			"routeVehicles": &graphql.Field{
				Type:        graphql.NewList(vehicleType),
				Description: "Live vehicle positions for a route",
				Args: graphql.FieldConfigArgument{
					"route_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					routeID := p.Args["route_id"].(string)
					return deps.Routes.GetLiveVehicles(p.Context, routeID)
				},
			},
			"stopDepartures": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
					Name: "Departure",
					Fields: graphql.Fields{
						"scheduled_time": &graphql.Field{Type: graphql.String},
						"platform":       &graphql.Field{Type: graphql.String},
						"trip": &graphql.Field{Type: graphql.NewObject(graphql.ObjectConfig{
							Name: "DepartureTrip",
							Fields: graphql.Fields{
								"id":       &graphql.Field{Type: graphql.String},
								"trip_id":  &graphql.Field{Type: graphql.String},
								"headsign": &graphql.Field{Type: graphql.String},
							},
						})},
					},
				})),
				Description: "Next departures at a stop",
				Args: graphql.FieldConfigArgument{
					"stop_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"limit":   &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 10},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					stopID := p.Args["stop_id"].(string)
					limit := p.Args["limit"].(int)
					deps, err := deps.Departures.NextDeparturesAtStop(p.Context, stopID, limit)
					if err != nil {
						return nil, err
					}
					// Convert domain.Departure to a map for GraphQL
					var result []map[string]interface{}
					for _, d := range deps {
						m := map[string]interface{}{
							"scheduled_time": d.ScheduledTime.Format("15:04:05"),
							"platform":       d.Platform,
						}
						if d.Trip != nil {
							m["trip"] = map[string]interface{}{
								"id":       d.Trip.ID,
								"trip_id":  d.Trip.TripID,
								"headsign": d.Trip.Headsign,
							}
						}
						result = append(result, m)
					}
					return result, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
}

// GraphQLHandler serves the GraphQL endpoint.
func GraphQLHandler(deps *Dependencies) fiber.Handler {
	schema, err := buildSchema(deps)
	if err != nil {
		// This would be a programming error in the schema definition
		panic("graphql schema build: " + err.Error())
	}

	type gqlRequest struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	return func(c *fiber.Ctx) error {
		var req gqlRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			OperationName:  req.OperationName,
			Context:        c.Context(),
		})

		return c.JSON(result)
	}
}

// Ensure domain types implement field resolvers for graphql-go via struct tags
var _ = domain.Stop{}
var _ = domain.Route{}
var _ = domain.Agency{}
