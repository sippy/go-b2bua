package rtp_proxy_types

import (
    "net"

    "github.com/sippy/go-b2bua/sippy/net"
)

type RtpProxyTransport interface {
    Address() net.Addr
    Get_rtpc_delay() float64
    Is_local() bool
    Send_command(string, func(string))
    Shutdown()
    Reconnect(net.Addr, *sippy_net.HostPort)
}
