package sippy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sippy/go-b2bua/sippy/conf"
	"github.com/sippy/go-b2bua/sippy/log"
	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/time"
)

type wireTestLogger struct{ messages []string }

func (l *wireTestLogger) Write(_ *sippy_time.MonoTime, _, message string) {
	l.messages = append(l.messages, message)
}

func newWireTestTM(logger *wireTestLogger) *sipTransactionManager {
	return &sipTransactionManager{
		config:   sippy_conf.NewConfig(sippy_log.NewErrorLogger(), logger),
		l1rcache: make(map[string]*sipTMRetransmitO),
		l2rcache: make(map[string]*sipTMRetransmitO),
	}
}

func takeWireMessage(t *testing.T, transport *test_sip_transport_factory) []byte {
	t.Helper()
	select {
	case data := <-transport.data_ch:
		return data
	default:
		t.Fatal("expected a transmitted message")
		return nil
	}
}

func TestSIPLogText(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""},
		{"OPTIONS\tSIP/2.0\r\n", "OPTIONS\tSIP/2.0\r\n"},
		{"Café �", "Café �"},
		{"\x00a\x1b\x7f Café\u202e\xff\xe2\x82", `\x00a\x1b\x7f Café\u202e\xff\xe2\x82`},
	} {
		if got := sipLogText([]byte(tc.input)); got != tc.want {
			t.Errorf("sipLogText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

type wireTestErrorLogger struct {
	sippy_log.ErrorLogger
	messages []string
}

func (l *wireTestErrorLogger) Error(args ...interface{}) {
	l.messages = append(l.messages, fmt.Sprint(args...))
}

func TestSIPBadMessageLog(t *testing.T) {
	logger := &wireTestErrorLogger{}
	tm := &sipTransactionManager{config: sippy_conf.NewConfig(logger, &wireTestLogger{})}
	tm.logBadMessage("bad request: \x1b", []byte("first\r\nsecond\xff\r\n"))
	const want = "bad request: \\x1b\nfirst\r\nsecond\\xff\r\n"
	if len(logger.messages) != 1 || logger.messages[0] != want {
		t.Fatalf("log entries = %q, want one entry %q", logger.messages, want)
	}
}

func TestSIPBadMessageLogTruncated(t *testing.T) {
	logger := &wireTestErrorLogger{}
	tm := &sipTransactionManager{config: sippy_conf.NewConfig(logger, &wireTestLogger{})}
	msg := strings.Repeat("a", sipBadLogFieldLimit)
	data := strings.Repeat("\xff", sipBadLogFieldLimit)
	tm.logBadMessage(msg+"omitted", []byte(data+"omitted"))
	const suffix = " … [truncated]"
	want := msg + suffix + "\n" + strings.Repeat(`\xff`, sipBadLogFieldLimit) + suffix
	if len(logger.messages) != 1 || logger.messages[0] != want {
		t.Fatal("expected one log entry with both fields truncated before escaping")
	}
}

var sipLogTextResult string

func BenchmarkSIPLogText(b *testing.B) {
	const msg = "OPTIONS sip:bob@example.com SIP/2.0\r\n" + testSIPHeaders + "\r\n" + testSDP
	for _, tc := range []struct{ name, input string }{
		{"ASCII", strings.ReplaceAll(msg, "Café", "Cafe")},
		{"UTF8", msg},
		{"Escaped", msg + "\xff\x00"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data := []byte(tc.input)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sipLogTextResult = sipLogText(data)
			}
		})
	}
}

func TestSIPWireLogAndRetransmit(t *testing.T) {
	logger := &wireTestLogger{}
	tm := newWireTestTM(logger)
	transport := NewTestSipTransportFactory()
	peer := sippy_net.NewHostPort("192.0.2.1", "5060")
	body := "Café\xff\x00\r\n"
	incoming := []byte("OPTIONS sip:bob@example.com SIP/2.0\r\n" + testSIPHeaders +
		"Content-Type: application/octet-stream\r\nContent-Length: 9\r\n\r\n" + body)
	outgoing := []byte("SIP/2.0 200 OK\r\n" + testSIPHeaders +
		"Content-Type: application/octet-stream\r\nContent-Length: 9\r\n\r\n" + body)
	sum := md5.Sum(incoming)
	key := hex.EncodeToString(sum[:])
	tm.transmitData(transport, outgoing, peer, key, "test@example.com", 0)
	if got := takeWireMessage(t, transport); !bytes.Equal(got, outgoing) {
		t.Fatalf("sent %q, want %q", got, outgoing)
	}
	tm.handleIncoming(incoming, peer, transport, nil)
	if got := takeWireMessage(t, transport); !bytes.Equal(got, outgoing) {
		t.Fatalf("retransmitted %q, want %q", got, outgoing)
	}
	if len(logger.messages) != 3 {
		t.Fatalf("expected send, receive, retransmit logs; got %v", logger.messages)
	}
	for _, msg := range logger.messages {
		if !utf8.ValidString(msg) || !strings.Contains(msg, "Café\\xff\\x00\r\n") {
			t.Errorf("unexpected SIP log: %q", msg)
		}
	}
}

func TestSIPRequestMissingMethod(t *testing.T) {
	config := sippy_conf.NewConfig(sippy_log.NewErrorLogger(), &wireTestLogger{})
	if req, err := parseSipRequest([]byte(" \t\r\n"+testSIPHeaders+"\r\n"), nil, config); req != nil || err == nil {
		t.Fatalf("expected a parse error and no partial request, got req=%v, err=%v", req != nil, err)
	}
}

func TestSIPRequestParseErrorResponse(t *testing.T) {
	const start = "OPTIONS sip:bob@example.com SIP/2.0\r\n"
	for _, tc := range []struct {
		name, wire string
		respond    bool
	}{
		{"truncated body", start + testSIPHeaders + "Content-Length: 10\r\n\r\nhello", true},
		{"bad header", start + "X-Test Missing Colon\r\n" + testSIPHeaders + "\r\n", true},
		{"missing Call-ID", start + strings.Replace(testSIPHeaders, "Call-ID: test@example.com\r\n", "", 1) + "\r\n", false},
		{"ACK", "ACK sip:bob@example.com SIP/2.0\r\n" + strings.Replace(testSIPHeaders, "CSeq: 1 OPTIONS", "CSeq: 1 ACK", 1) + "Content-Length: 1\r\n\r\n", false},
		{"response", "SIP/2.0 200 OK\r\nX-Test Missing Colon\r\n" + testSIPHeaders + "\r\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := &wireTestLogger{}
			tm := newWireTestTM(logger)
			transport := NewTestSipTransportFactory()
			peer := sippy_net.NewHostPort("192.0.2.1", "5060")
			tm.handleIncoming([]byte(tc.wire), peer, transport, nil)
			if !tc.respond {
				select {
				case data := <-transport.data_ch:
					t.Fatalf("unexpected response: %q", data)
				default:
				}
				return
			}
			data := takeWireMessage(t, transport)
			resp, err := ParseSipResponse(data, nil, tm.config)
			if err != nil {
				t.Fatal(err)
			}
			tid, err := resp.GetTId(true, true, true)
			if err != nil {
				t.Fatal(err)
			}
			if resp.scode != 400 || resp.GetBody() != nil || tid.CallId != "test@example.com" ||
				tid.Branch != "z9hG4bK-test" || tid.FromTag != "test" || tid.CSeqMethod != "OPTIONS" || tid.CSeq != "1" {
				t.Fatalf("unexpected 400 response: %s", data)
			}
			if len(logger.messages) != 2 || !strings.HasPrefix(logger.messages[0], "RECEIVED") || !strings.HasPrefix(logger.messages[1], "SENDING") {
				t.Fatalf("unexpected receive/send order: %v", logger.messages)
			}
			tm.handleIncoming([]byte(tc.wire), peer, transport, nil)
			if got := takeWireMessage(t, transport); !bytes.Equal(got, data) {
				t.Fatalf("retransmitted %q, want %q", got, data)
			}
		})
	}
}
