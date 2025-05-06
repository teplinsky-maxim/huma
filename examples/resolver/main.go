// This example shows how to use resolvers to provide additional validation
// for params and body fields, and how exhaustive errors are returned.
//
//	# Example call returning seven errors
//	restish put :8888/count/3?count=15 -H Count:-3 count:9, nested.subCount: 6
//
//	# Example success
//	restish put :8888/count/1 count:2, nested.subCount: 4
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.
	"github.com/teplinsky-maxim/huma/v2"
	"github.com/teplinsky-maxim/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	_ "github.com/teplinsky-maxim/huma/v2/formats/cbor"
)

// Options for the CLI.
type Options struct {
	Port int `doc:"Port to listen on." short:"p" default:"8888"`
}

// Create a new input type with additional validation attached to it.
type IntNot3 int

// Resolve is called by Huma to validate the input. Prefix is the current
// path like `path.to[3].field`, e.g. `query.count` or `body.nested.subCount`.
// Resolvers can also be attached to structs to provide validation across
// multiple field combinations, e.g. "if foo is set then bar must be a
// multiple of foo's value". Use `prefix.With("bar")` in that scenario.
func (i IntNot3) Resolve(ctx huma.Context, prefix *huma.PathBuffer) []error {
	if i != 0 && i%3 == 0 {
		return []error{&huma.ErrorDetail{
			Location: prefix.String(),
			Message:  "Value cannot be a multiple of three",
			Value:    i,
		}}
	}
	return nil
}

// Ensure our resolver meets the expected interface.
var _ huma.ResolverWithPath = (*IntNot3)(nil)

func main() {
	// Create the CLI, passing a function to be called with your custom options
	// after they have been parsed.
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		router := chi.NewMux()

		api := humachi.New(router, huma.DefaultConfig("My API", "1.0.0"))

		// Register the greeting operation.
		huma.Register(api, huma.Operation{
			OperationID: "put-count",
			Summary:     "Put a count of things",
			Method:      http.MethodPut,
			Path:        "/count/{count}",
		}, func(ctx context.Context, input *struct {
			PathCount   IntNot3 `path:"count" example:"2" minimum:"1" maximum:"10"`
			QueryCount  IntNot3 `query:"count" example:"2" minimum:"1" maximum:"10"`
			HeaderCount IntNot3 `header:"Count" example:"2" minimum:"1" maximum:"10"`
			Body        struct {
				Count  IntNot3 `json:"count" example:"2" minimum:"1" maximum:"10"`
				Nested *struct {
					SubCount IntNot3 `json:"subCount" example:"2" minimum:"1" maximum:"10"`
				} `json:"nested,omitempty"`
			}
		}) (*struct{}, error) {
			fmt.Printf("Got input: %+v\n", input)
			return nil, nil
		})

		// Tell the CLI how to start your router.
		hooks.OnStart(func() {
			// Start the server
			http.ListenAndServe(fmt.Sprintf(":%d", options.Port), router)
		})
	})

	// Run the CLI. When passed no commands, it starts the server.
	cli.Run()
}
