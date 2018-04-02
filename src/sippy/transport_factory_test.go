package sippy

import (
    "strings"

    "sippy/net"
    "sippy/time"
)

type test_sip_transport_factory struct {
    recv_cb     sippy_net.DataPacketReceiver
    laddress    *sippy_net.HostPort
    data_ch     chan []byte
}

func NewTestSipTransportFactory() *test_sip_transport_factory {
    return &test_sip_transport_factory{
        laddress    : sippy_net.NewHostPort("0.0.0.0", "5060"),
        data_ch     : make(chan []byte, 100),
    }
}

func (self *test_sip_transport_factory) NewSipTransport(addr *sippy_net.HostPort, recv_cb sippy_net.DataPacketReceiver) (sippy_net.Transport, error) {
    self.recv_cb = recv_cb
    return self, nil
}

func (self *test_sip_transport_factory) GetLAddress() *sippy_net.HostPort {
    return self.laddress
}

func (self *test_sip_transport_factory) SendTo(data []byte, dest *sippy_net.HostPort) {
    self.data_ch <- data
}

func (self *test_sip_transport_factory) SendToWithCb(data []byte, dest *sippy_net.HostPort, cb func()) {
    self.SendTo(data, dest)
    if cb != nil {
        cb()
    }
}

func (self *test_sip_transport_factory) Shutdown() {
}

func (self *test_sip_transport_factory) feed(inp []string) {
    s := strings.Join(inp, "\r\n")
    rtime, _ := sippy_time.NewMonoTime()
    self.recv_cb([]byte(s), sippy_net.NewHostPort("1.1.1.1", "5060"), self, rtime)
}

func (self *test_sip_transport_factory) get() []byte {
    return <-self.data_ch
}
