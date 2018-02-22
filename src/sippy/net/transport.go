package sippy_net

import (
    "sippy/time"
)

type SipPacketReceiver func(data []byte, addr *HostPort, server Transport, rtime *sippy_time.MonoTime)

type SipTransportFactory interface {
    NewSipTransport(*HostPort, SipPacketReceiver) (Transport, error)
}

type Transport interface {
    Shutdown()
    GetLAddress() *HostPort
    SendTo([]byte, *HostPort)
    SendToWithCb([]byte, *HostPort, func())
}
