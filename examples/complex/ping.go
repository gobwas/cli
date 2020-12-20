package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gobwas/cli"
)

type ping struct {
	core *core

	interval time.Duration
	method   string
}

func (p *ping) DefineFlags(fs *flag.FlagSet) {
	fs.DurationVar(&p.interval,
		"interval", time.Second,
		"how frequent to ping",
	)
	fs.StringVar(&p.method,
		"method", "HEAD",
		"method to be used for ping",
	)
}

func (p *ping) Synopsis() string {
	return "<host>[ host...]"
}

func (p *ping) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return cli.Exitf(2, "no hosts to ping were given")
	}
	for {
		for _, host := range args {
			req, err := http.NewRequest(p.method, host, nil)
			if err != nil {
				return err
			}
			if p.core.verbose {
				dump, _ := httputil.DumpRequest(req, false)
				fmt.Println(string(dump))
			}
			res, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				return err
			}
			if p.core.verbose {
				dump, _ := httputil.DumpResponse(res, false)
				fmt.Println(string(dump))
			} else {
				fmt.Print(".")
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.interval):
		}
	}
}
