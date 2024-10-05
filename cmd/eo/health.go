package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
)

var cmdHealth = &health{}

type health struct {
	endpoint string
}

func (c *health) parse(args []string, _ func(string) string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.StringVar(&c.endpoint, "endpoint", "http://localhost/health", "http health check endpoint")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `
The health command is used to check if the container is ready.

Usage:
	%s health [arguments]

Arguments:
`[1:], os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	} else if fs.NArg() != 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	return nil
}

func (c *health) run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()
	// we don't need the backoff since docker will handle that for us
	hc := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("failed making request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("not ready")
	}
	return nil
}
