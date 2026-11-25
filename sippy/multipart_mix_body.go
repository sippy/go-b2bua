package sippy

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/types"
)

// MultipartMixBody is a parsed multipart/mixed entity. Parsing does not
// change the source MsgBody or how it is forwarded.
type MultipartMixBody struct {
	ContentType string
	Boundary    string
	Parts       []MultipartMixPart
}

// MultipartMixPart holds a part's MIME headers and its unparsed entity body.
type MultipartMixPart struct {
	Headers textproto.MIMEHeader
	Body    sippy_types.MsgBody
}

// ParseMultipartMixed exposes the parts of a multipart/mixed SIP body without
// decoding transfer encodings or automatically treating embedded SDP as an offer.
func ParseMultipartMixed(body sippy_types.MsgBody) (*MultipartMixBody, error) {
	contentType := body.GetMtype()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid multipart Content-Type: %w", err)
	}
	if mediaType != "multipart/mixed" || params["boundary"] == "" {
		return nil, fmt.Errorf("expected multipart/mixed with a boundary")
	}
	mixed := &MultipartMixBody{ContentType: contentType, Boundary: params["boundary"]}
	reader := multipart.NewReader(strings.NewReader(body.String()), mixed.Boundary)
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF && len(mixed.Parts) > 0 {
			return mixed, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read multipart part: %w", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("read multipart payload: %w", err)
		}
		partType := part.Header.Get("Content-Type")
		if partType == "" {
			partType = "text/plain"
		}
		mixed.Parts = append(mixed.Parts, MultipartMixPart{
			Headers: part.Header,
			Body:    NewMsgBody(string(data), partType),
		})
	}
}

// ToMsgBody serializes the current parts as a new SIP body. Untouched messages
// should use the original MsgBody to retain the exact MIME framing.
func (mixed *MultipartMixBody) ToMsgBody(localAddr *sippy_net.HostPort) (sippy_types.MsgBody, error) {
	if len(mixed.Parts) == 0 {
		return nil, fmt.Errorf("multipart/mixed requires at least one part")
	}
	var data strings.Builder
	writer := multipart.NewWriter(&data)
	if err := writer.SetBoundary(mixed.Boundary); err != nil {
		return nil, err
	}
	mediaType, params, err := mime.ParseMediaType(mixed.ContentType)
	if err != nil || mediaType != "multipart/mixed" {
		return nil, fmt.Errorf("invalid multipart/mixed Content-Type")
	}
	params["boundary"] = mixed.Boundary
	for _, part := range mixed.Parts {
		headers := textproto.MIMEHeader{}
		for name, values := range part.Headers {
			headers[name] = values
		}
		if mtype := part.Body.GetMtype(); mtype != "text/plain" || headers.Get("Content-Type") != "" {
			headers.Set("Content-Type", mtype)
		}
		out, err := writer.CreatePart(headers)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(out, part.Body.LocalStr(localAddr)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return NewMsgBody(data.String(), mime.FormatMediaType(mediaType, params)), nil
}
