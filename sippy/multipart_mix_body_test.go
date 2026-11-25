package sippy

import (
	"mime"
	"strings"
	"testing"
)

func TestMultipartMixed(t *testing.T) {
	const contentType = `Multipart/Mixed; boundary="MiX-7"; x-test=keep`
	const opaque = "\xff\x00\r\n--MiX-7-not-a-delimiter\r\n=41"
	const wire = "preamble\r\n--MiX-7\r\nContent-Type: application/sdp\r\n\r\n" + testSDP +
		"\r\n--MiX-7\r\nContent-Type: application/octet-stream\r\nContent-Transfer-Encoding: binary\r\n" +
		"Content-ID: <opaque>\r\n\r\n" + opaque +
		"\r\n--MiX-7\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n=41\r\n--MiX-7--\r\nepilogue"
	original := NewMsgBody(wire, contentType)
	mixed, err := ParseMultipartMixed(original)
	if err != nil {
		t.Fatal(err)
	}
	if mixed.Boundary != "MiX-7" || mixed.ContentType != contentType || len(mixed.Parts) != 3 {
		t.Fatalf("unexpected multipart metadata: %+v", mixed)
	}
	if _, err := original.GetSdp(); err == nil {
		t.Fatal("multipart was treated as a top-level SDP offer")
	}
	sdp, sdpErr := mixed.Parts[0].Body.GetSdp()
	if sdpErr != nil {
		t.Fatal(sdpErr)
	}
	sdp.AppendAHeader("sendonly")
	if original.String() != wire {
		t.Fatal("parsing or editing parts changed the original entity")
	}
	rebuilt, err := mixed.ToMsgBody(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(rebuilt.GetMtype())
	if err != nil || params["boundary"] != "MiX-7" || params["x-test"] != "keep" {
		t.Fatalf("wrong rebuilt Content-Type: %s", rebuilt.GetMtype())
	}
	roundTrip, err := ParseMultipartMixed(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.Parts) != 3 || !strings.Contains(roundTrip.Parts[0].Body.String(), "a=sendonly\r\n") {
		t.Fatal("SDP edit did not survive serialization")
	}
	if part := roundTrip.Parts[1]; part.Body.String() != opaque || part.Headers.Get("Content-ID") != "<opaque>" {
		t.Fatal("opaque part or its headers changed")
	}
	if part := roundTrip.Parts[2]; part.Body.String() != "=41" || part.Body.GetMtype() != "text/plain" ||
		part.Headers.Get("Content-Transfer-Encoding") != "quoted-printable" || part.Headers.Get("Content-Type") != "" {
		t.Fatal("encoded part was decoded or its headers changed")
	}
}

func TestMultipartMixedMalformed(t *testing.T) {
	for _, tc := range []struct{ contentType, wire string }{
		{"multipart/alternative; boundary=sep", "--sep\r\n\r\ndata\r\n--sep--\r\n"},
		{"multipart/mixed", "--sep\r\n\r\ndata\r\n--sep--\r\n"},
		{"multipart/mixed; boundary=sep", "--sep\r\n\r\ndata"},
		{"multipart/mixed; boundary=sep", "--sep\r\nnot a header\r\n\r\ndata\r\n--sep--\r\n"},
		{"multipart/mixed; boundary=sep", "--sep--\r\n"},
	} {
		if _, err := ParseMultipartMixed(NewMsgBody(tc.wire, tc.contentType)); err == nil {
			t.Errorf("parsed malformed or non-mixed entity %q, %q", tc.contentType, tc.wire)
		}
	}
}
