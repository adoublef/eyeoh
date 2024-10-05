package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"go.adoublef/eyeoh/internal/fs"
	"go.adoublef/eyeoh/internal/net/http"
	"go.adoublef/eyeoh/internal/time/rate"
	"golang.org/x/sync/errgroup"
)

var cmdServe = &serve{}

type serve struct {
	addr                                   string
	rateLimit                              rate.Rate
	readTimeout, writeTimeout, idleTimeout time.Duration
	maxHeaderBytes                         int
}

func (c *serve) parse(args []string, getenv func(string) string) error {
	// note: https://grafana.com/docs/agent/latest/static/configuration/flags/
	// note: https://clig.dev/#arguments-and-flags
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&c.addr, "http-address", "0.0.0.0:0", "http listening address")
	// Cloudflare sets a 1000/min rate limit default
	// throttle safe requests and limit non-safe requests
	fs.TextVar(&c.rateLimit, "rate-limit", rate.Rate{N: 1000, D: time.Minute}, "api rate limit")
	fs.IntVar(&c.maxHeaderBytes, "max-header-bytes", http.DefaultMaxHeaderBytes, "max request header size in bytes")
	fs.DurationVar(&c.readTimeout, "read-timeout", http.DefaultReadTimeout, "max duration for reading request body")
	fs.DurationVar(&c.writeTimeout, "write-timeout", http.DefaultWriteTimeout, "max duration for writing response")
	fs.DurationVar(&c.idleTimeout, "idle-timeout", http.DefaultIdleTimeout, "max idle time between requests")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `
The serve command initialises and runs a HTTP server.

Usage:
	%s serve [arguments]

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

func (c *serve) run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()

	// this can be handle by a flag?
	shutdown, err := setupOTel(ctx)
	if err != nil {
		return err
	}
	defer shutdown(ctx)

	// create a grpc client that will be passed into the [http.Handler]
	fsys := &fs.FS{ /* FIXME */ }

	hs := &http.Server{
		Addr:           c.addr,
		Handler:        http.Handler(c.rateLimit.N, c.rateLimit.D, fsys),
		BaseContext:    func(l net.Listener) context.Context { return ctx },
		MaxHeaderBytes: c.maxHeaderBytes,
		// todo: ReadHeaderTimeout uses ReadTimeout if not set
		ReadTimeout:  c.readTimeout,
		WriteTimeout: c.writeTimeout,
		IdleTimeout:  c.idleTimeout,
	}
	hs.RegisterOnShutdown(cancel)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		// http start
		switch {
		case hs.TLSConfig != nil:
			err = hs.ListenAndServeTLS("", "")
		default:
			err = hs.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	eg.Go(func() error {
		// http close
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := hs.Shutdown(ctx)
		if err != nil {
			err = errors.Join(hs.Close())
		}
		return err
	})

	return eg.Wait()
}
