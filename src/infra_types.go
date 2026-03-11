package main

import "bytes"

type infraRunContext struct {
	Addr string
	Host string
	WG   wgConfig
}

type localExecFunc func(name string, args ...string) (string, error)

type remoteFileSession interface {
	SetStdin(reader *bytes.Reader)
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
}

type infraStepFunc func(infraRunContext) error
