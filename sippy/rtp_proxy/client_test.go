package rtp_proxy

import (
	"net"
	"strings"
	"sync"
	"testing"

	sippy_net "github.com/sippy/go-b2bua/sippy/net"
)

type fakeRtppTransport struct {
	lock      sync.Mutex
	commands  []string
	callbacks []func(string)
}

func (self *fakeRtppTransport) Address() net.Addr {
	return &net.UDPAddr{}
}

func (self *fakeRtppTransport) Get_rtpc_delay() float64 {
	return 0
}

func (self *fakeRtppTransport) Is_local() bool {
	return false
}

func (self *fakeRtppTransport) Send_command(command string, result_callback func(string)) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.commands = append(self.commands, command)
	if strings.HasPrefix(command, "VF ") {
		self.callbacks = append(self.callbacks, result_callback)
	}
}

func (self *fakeRtppTransport) Shutdown() {
}

func (self *fakeRtppTransport) Reconnect(net.Addr, *sippy_net.HostPort) {
}

func (self *fakeRtppTransport) commandCount(prefix string) int {
	self.lock.Lock()
	defer self.lock.Unlock()
	n := 0
	for _, command := range self.commands {
		if strings.HasPrefix(command, prefix) {
			n += 1
		}
	}
	return n
}

func (self *fakeRtppTransport) capCallbacks() []func(string) {
	self.lock.Lock()
	defer self.lock.Unlock()
	callbacks := make([]func(string), len(self.callbacks))
	copy(callbacks, self.callbacks)
	return callbacks
}

func TestGoOnlineStartsSingleConcurrentCapsCheck(t *testing.T) {
	transport := &fakeRtppTransport{}
	rtpc := NewRtp_proxy_client_base(nil, &rtpProxyClientOpts{})
	rtpc.transport = transport

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rtpc.GoOnline()
		}()
	}
	wg.Wait()

	callbacks := transport.capCallbacks()
	if len(callbacks) != 5 {
		t.Fatalf("expected 5 capability queries, got %d", len(callbacks))
	}
	if n := transport.commandCount("VF "); n != 5 {
		t.Fatalf("expected one capability query batch, got %d VF commands", n)
	}

	wg = sync.WaitGroup{}
	for _, callback := range callbacks {
		wg.Add(1)
		go func(callback func(string)) {
			defer wg.Done()
			callback("1")
		}(callback)
	}
	wg.Wait()

	if !rtpc.IsOnline() {
		t.Fatal("expected client to go online after capability replies")
	}
	if !rtpc.SBindSupported() || !rtpc.TNotSupported() || !rtpc.WdntSupported() {
		t.Fatal("expected capability flags to be set from successful replies")
	}
	if n := transport.commandCount("Ib"); n != 1 {
		t.Fatalf("expected one heartbeat after going online, got %d", n)
	}
}
