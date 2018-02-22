package sippy

import (
    "sippy/conf"
    "sippy/net"
)

type default_sip_transport_factory struct {
    config  sippy_conf.Config
}

func NewDefaultSipTransportFactory(config sippy_conf.Config) *default_sip_transport_factory {
    return &default_sip_transport_factory{
        config  : config,
    }
}

func (self *default_sip_transport_factory) NewSipTransport(laddress *sippy_net.HostPort, handler sippy_net.SipPacketReceiver) (sippy_net.Transport, error) {
    sopts := NewUdpServerOpts(laddress, handler)
    return NewUdpServer(self.config, sopts)
}
