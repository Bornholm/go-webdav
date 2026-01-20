package main

import (
	"context"
	"net"

	"github.com/grandcat/zeroconf"
	"github.com/pkg/errors"
)

const (
	MDNSService = "_webdav._tcp"
	MDNSDomain  = "local."
)

func startAnnouncingService(ctx context.Context, port int) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return errors.WithStack(err)
	}

	server, err := zeroconf.Register("GoWebDAV", MDNSService, MDNSDomain, port, []string{}, ifaces)
	if err != nil {
		return errors.WithStack(err)
	}

	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()

	return nil
}
