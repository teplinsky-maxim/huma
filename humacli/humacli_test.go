package humacli_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/teplinsky-maxim/huma/v2"
	"github.com/teplinsky-maxim/huma/v2/adapters/humago"
	"github.com/teplinsky-maxim/huma/v2/humacli"
)

func ExampleCLI() {
	// First, define your input options.
	type Options struct {
		Debug bool   `doc:"Enable debug logging"`
		Host  string `doc:"Hostname to listen on."`
		Port  int    `doc:"Port to listen on." short:"p" default:"8888"`
	}

	// Then, create the CLI.
	cli := humacli.New(func(hooks humacli.Hooks, opts *Options) {
		fmt.Printf("Options are debug:%v host:%v port%v\n",
			opts.Debug, opts.Host, opts.Port)

		// Set up the router & API
		mux := http.NewServeMux()
		api := humago.New(mux, huma.DefaultConfig("My API", "1.0.0"))

		huma.Register(api, huma.Operation{
			OperationID: "hello",
			Method:      http.MethodGet,
			Path:        "/hello",
		}, func(ctx context.Context, input *struct{}) (*struct{}, error) {
			// TODO: implement handler
			return nil, nil
		})

		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%d", opts.Host, opts.Port),
			Handler: mux,
			// TODO: Set up timeouts!
		}

		hooks.OnStart(func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		})

		hooks.OnStop(func() {
			srv.Shutdown(context.Background())
		})
	})

	// Run the thing!
	cli.Run()
}

func TestCLIPlain(t *testing.T) {
	type Options struct {
		Debug bool
		Host  string
		Port  int

		// ignore private fields, should not crash.
		ignore bool
	}

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		assert.True(t, options.Debug)
		assert.Equal(t, "localhost", options.Host)
		assert.Equal(t, 8001, options.Port)
		assert.False(t, options.ignore)
		hooks.OnStart(func() {
			// Do nothing
		})
	})

	cli.Root().SetArgs([]string{"--debug", "--host", "localhost", "--port", "8001"})
	cli.Run()
}

func TestCLIEnv(t *testing.T) {
	type Options struct {
		Debug bool
		Host  string
		Port  int
	}

	t.Setenv("SERVICE_DEBUG", "true")
	t.Setenv("SERVICE_HOST", "localhost")
	t.Setenv("SERVICE_PORT", "8001")

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		assert.True(t, options.Debug)
		assert.Equal(t, "localhost", options.Host)
		assert.Equal(t, 8001, options.Port)
		hooks.OnStart(func() {
			// Do nothing
		})
	})

	cli.Root().SetArgs([]string{})
	cli.Run()
}

func TestCLIAdvanced(t *testing.T) {
	type DebugOption struct {
		Debug bool `doc:"Enable debug mode." default:"false"`
	}

	type Options struct {
		// Example of option composition via embedded type.
		DebugOption
		Host    string        `doc:"Hostname to listen on."`
		Port    *int          `doc:"Port to listen on." short:"p" default:"8000"`
		Timeout time.Duration `doc:"Request timeout." default:"5s"`
	}

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		assert.True(t, options.Debug)
		assert.Equal(t, "localhost", options.Host)
		assert.Equal(t, 8001, *options.Port)
		assert.Equal(t, 10*time.Second, options.Timeout)
		hooks.OnStart(func() {
			// Do nothing
		})
	})

	// A custom pre-run isn't overwritten and should still work!
	customPreRun := false
	cli.Root().PersistentPreRun = func(cmd *cobra.Command, args []string) {
		customPreRun = true
	}

	cli.Root().SetArgs([]string{"--debug", "--host", "localhost", "--port", "8001", "--timeout", "10s"})
	cli.Run()
	assert.True(t, customPreRun)
}

func TestCLIHelp(t *testing.T) {
	type Options struct {
		Debug bool
		Host  string
		Port  int
	}

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		// Do nothing
	})

	cli.Root().Use = "myapp"
	cli.Root().SetArgs([]string{"--help"})
	buf := bytes.NewBuffer(nil)
	cli.Root().SetOut(buf)
	cli.Root().SetErr(buf)
	cli.Run()

	assert.Equal(t, "Usage:\n  myapp [flags]\n\nFlags:\n      --debug         \n  -h, --help          help for myapp\n      --host string   \n      --port int\n", buf.String())
}

func TestCLICommandWithOptions(t *testing.T) {
	type Options struct {
		Debug bool
	}

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		// Do nothing
	})

	wasSet := false
	cli.Root().AddCommand(&cobra.Command{
		Use: "custom",
		Run: humacli.WithOptions(func(cmd *cobra.Command, args []string, options *Options) {
			if options.Debug {
				wasSet = true
			}
		}),
	})

	cli.Root().SetArgs([]string{"custom", "--debug"})
	cli.Run()

	assert.True(t, wasSet)
}

func TestCLIShutdown(t *testing.T) {
	type Options struct{}

	started := make(chan bool, 1)
	stopping := make(chan bool, 1)
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		hooks.OnStart(func() {
			started <- true
			<-stopping
		})
		hooks.OnStop(func() {
			stopping <- true
		})
	})

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find process: %v", os.Getpid())
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		p.Signal(os.Interrupt)
	}()

	cli.Root().SetArgs([]string{})
	cli.Run()
	assert.True(t, <-started)
}

func TestCLIBadType(t *testing.T) {
	type Options struct {
		Debug []struct{}
	}

	assert.Panics(t, func() {
		humacli.New(func(hooks humacli.Hooks, options *Options) {})
	})
}

func TestCLIBadDefaults(t *testing.T) {
	type OptionsBool struct {
		Debug bool `default:"notabool"`
	}

	type OptionsInt struct {
		Debug int `default:"notanint"`
	}

	assert.Panics(t, func() {
		humacli.New(func(hooks humacli.Hooks, options *OptionsBool) {})
	})

	assert.Panics(t, func() {
		humacli.New(func(hooks humacli.Hooks, options *OptionsInt) {})
	})
}
