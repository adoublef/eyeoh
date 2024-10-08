package main

import (
	"context"
	"flag"
	"net/http"
	"testing"
	"time"

	"go.adoublef/eyeoh/internal/testing/is"
	"go.adoublef/eyeoh/internal/testing/wait"
	"golang.org/x/sync/errgroup"
)

func Test_serve_parse(t *testing.T) {
	type testcase struct {
		in   []string
		want error
	}

	var tt = map[string]testcase{
		"OKRate": {
			in: []string{"--rate-limit", "1/10s"},
		},
		"ErrTooManyArgs": {
			in:   []string{"never"},
			want: flag.ErrHelp,
		},
	}
	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			err := (&serve{}).parse(tc.in, nil)
			is.NotOK(t, err, tc.want) // got;want
		})
	}
}

func Test_serve_run(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		var s serve
		// random port?
		err := s.parse([]string{"--http-address", ":8080"}, nil)
		is.OK(t, err)
		// cancellable context is needed
		ctx, cancel := context.WithCancel(context.Background())
		// errgroup makes handling errors in multiple go routines easier
		eg, ctx := errgroup.WithContext(ctx)
		// 1. start service
		eg.Go(func() error { return s.run(ctx) })
		// 1. wait for service
		eg.Go(func() error {
			defer cancel()
			err := wait.ForHTTP(ctx, 30*time.Second, "http://localhost:8080/ready", options)
			if err != nil {
				return err
			}
			return nil
		})
		is.OK(t, eg.Wait()) // service is ready
	})
}

var options = func(r *http.Request) {
	// need accept header
	r.Header.Set("Accept", "*/*")
}
