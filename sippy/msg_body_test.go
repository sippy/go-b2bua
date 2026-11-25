package sippy

import "testing"

const testSDP = "v=0\r\no=user 1 1 IN IP4 192.0.2.1\r\ns=Café\r\nc=IN IP4 192.0.2.1\r\nt=0 0\r\nm=audio 49170 RTP/AVP 0\r\n"

func TestMsgBodySDPMediaType(t *testing.T) {
	const contentType = `Application/SDP ; charset="UTF-8"`
	body := NewMsgBody(testSDP, contentType)
	if !IsSdpBody(body) {
		t.Fatal("did not recognize parameterized SDP")
	}
	if body.String() != testSDP || body.GetMtype() != contentType {
		t.Fatal("unparsed SDP changed")
	}
	if _, err := body.GetSdp(); err != nil {
		t.Fatal(err)
	}
	if body.GetMtype() != contentType {
		t.Fatal("SDP interpretation changed Content-Type")
	}
}

func TestMsgBodyOpaque(t *testing.T) {
	const boundary = "MiXeD"
	multipart := "--" + boundary + "\r\nContent-Type: application/sdp\r\n\r\n" + testSDP +
		"\r\n--" + boundary + "\r\nContent-Type: application/octet-stream\r\n\r\n\xff\x00\r\n--" + boundary + "--\r\n"
	for _, tc := range []struct{ contentType, content string }{
		{"application/octet-stream", "\xff\x00Café\n"},
		{`Multipart/Mixed; boundary="MiXeD"`, multipart},
	} {
		t.Run(tc.contentType, func(t *testing.T) {
			body := NewMsgBody(tc.content, tc.contentType)
			if IsSdpBody(body) {
				t.Fatal("opaque MIME entity classified as SDP")
			}
			if err := NewSdpSession().FixupVersion(body); err != nil {
				t.Fatal(err)
			}
			if _, err := body.GetSdp(); err == nil {
				t.Fatal("GetSdp accepted an opaque MIME entity")
			}
			body.AppendAHeader("nated:yes")
			if copy := body.GetCopy(); copy.String() != tc.content || copy.GetMtype() != tc.contentType {
				t.Fatal("SDP operations changed the opaque entity")
			}
		})
	}
}

func TestMsgBodyInvalidSDPEncoding(t *testing.T) {
	content := testSDP + "a=invalid:\xff\r\n"
	body := NewMsgBody(content, "application/sdp")
	if _, err := body.GetSdp(); err == nil {
		t.Fatal("GetSdp accepted invalid UTF-8")
	}
	if body.String() != content {
		t.Fatal("failed SDP parsing changed the body")
	}
}
