package sippy

import (
	"strings"
	"testing"

	"github.com/sippy/go-b2bua/sippy/net"
)

func TestSIPLossEmulDirections(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode LossEmulMode
	}{
		{"recv", LossEmulRecv},
		{"send", LossEmulSend},
		{"both", LossEmulBoth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := &wireTestLogger{}
			tm := newWireTestTM(logger)
			transport := NewTestSipTransportFactory()
			peer := sippy_net.NewHostPort("192.0.2.1", "5060")
			if err := tm.SetLossEmul(tc.mode, 1); err != nil {
				t.Fatal(err)
			}
			outgoing := []byte("SIP/2.0 200 OK\r\n" + testSIPHeaders + "\r\n")
			tm.transmitData(transport, outgoing, peer, "", "", 0)
			if tc.mode&LossEmulSend != 0 {
				assertSIPLossLog(t, logger, "DISCARDING message to ")
				select {
				case <-transport.data_ch:
					t.Fatal("discarded packet reached the transport")
				default:
				}
			} else {
				assertSIPLossLog(t, logger, "SENDING message to ")
				takeWireMessage(t, transport)
			}
			tm.transmitData(transport, outgoing, peer, "", "", 0)
			takeWireMessage(t, transport)

			// A correlatable request with a short body produces a 400 when received.
			incoming := []byte("OPTIONS sip:bob@example.com SIP/2.0\r\n" + testSIPHeaders +
				"Content-Length: 10\r\n\r\nhello")
			if tc.mode&LossEmulRecv != 0 {
				tm.handleIncoming(incoming, peer, transport, nil)
				assertSIPLossLog(t, logger, "DISCARDING message from ")
				if len(tm.l1rcache) != 0 || len(tm.l2rcache) != 0 || len(transport.data_ch) != 0 {
					t.Fatal("receive discard reached the cache or generated a reply")
				}
			}
			tm.handleIncoming(incoming, peer, transport, nil)
			if reply := string(takeWireMessage(t, transport)); !strings.HasPrefix(reply, "SIP/2.0 400 ") {
				t.Fatalf("unexpected reply after receiving request: %q", reply)
			}
		})
	}
}

func assertSIPLossLog(t *testing.T, logger *wireTestLogger, prefix string) {
	t.Helper()
	if n := len(logger.messages); n == 0 || !strings.HasPrefix(logger.messages[n-1], prefix) {
		t.Fatalf("expected log prefix %q, got %q", prefix, logger.messages)
	}
}
