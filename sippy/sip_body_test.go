package sippy

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sippy/go-b2bua/sippy/conf"
	"github.com/sippy/go-b2bua/sippy/log"
	"github.com/sippy/go-b2bua/sippy/types"
)

const testSIPHeaders = "Via: SIP/2.0/UDP 192.0.2.1:5060;branch=z9hG4bK-test\r\n" +
	"From: <sip:alice@example.com>;tag=test\r\nTo: <sip:bob@example.com>\r\n" +
	"Call-ID: test@example.com\r\nCSeq: 1 OPTIONS\r\n"

func TestSIPBodyOctets(t *testing.T) {
	config := sippy_conf.NewConfig(sippy_log.NewErrorLogger(), NewTestSipLogger())
	for _, tc := range []struct {
		name, contentType, payload, delimiter, expectedType string
	}{
		{"UTF-8", "Content-Type: Text/Plain\r\n", "Café ☕\r\n", "\r\n", "Text/Plain"},
		{"opaque", "c: Application/Octet-Stream\r\n", "\xff\x00\r\n\r\nbody\r\n", "\r\n", "Application/Octet-Stream"},
		{"undeclared binary", "", "binary\xff", "\r\n", "application/octet-stream"},
		{"LF framing", "Content-Type: text/plain\r\n", "Café\r\n\r\nbody", "\n", "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, start := range []string{"OPTIONS sip:bob@example.com SIP/2.0", "SIP/2.0 200 OK"} {
				length := strconv.Itoa(len(tc.payload))
				header := start + "\r\n" + testSIPHeaders + tc.contentType + "l: " + length + "\r\n"
				if tc.delimiter == "\n" {
					header = strings.ReplaceAll(header, "\r\n", "\n")
				}
				wire := []byte(header + tc.delimiter + tc.payload + "trailing octets")
				var msg interface {
					sippy_types.SipMsg
					Bytes() []byte
				}
				var err error
				if strings.HasPrefix(start, "SIP/2.0") {
					msg, err = ParseSipResponse(wire, nil, config)
				} else {
					msg, err = ParseSipRequest(wire, nil, config)
				}
				if err != nil {
					t.Fatal(err)
				}
				if body := msg.GetBody(); body == nil || body.String() != tc.payload || body.GetMtype() != tc.expectedType {
					t.Fatalf("unexpected parsed body: %#v", body)
				}
				for _, serialized := range []struct{ wire, lengthHeader string }{
					{string(msg.Bytes()), "Content-Length"},
					{msg.LocalStr(nil, false), "Content-Length"},
					{msg.LocalStr(nil, true), "l"},
				} {
					hdrs, body, found := strings.Cut(serialized.wire, "\r\n\r\n")
					if !found || body != tc.payload || !strings.Contains(hdrs+"\r\n", "\r\n"+serialized.lengthHeader+": "+length+"\r\n") {
						t.Errorf("incorrect serialization: %q", serialized.wire)
					}
				}
			}
		})
	}
}
