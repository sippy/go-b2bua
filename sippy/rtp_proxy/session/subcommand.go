// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2022 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2024 Sippy Software, Inc. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
// list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package rtp_proxy_session

import (
	"fmt"
	"strings"

	"github.com/sippy/go-b2bua/sippy/exceptions"
	"github.com/sippy/go-b2bua/sippy/sdp"
	"github.com/sippy/go-b2bua/sippy/types"
)

var (
	dtlsTransports = []string{"UDP/TLS/RTP/SAVP", "UDP/TLS/RTP/SAVPF"}
	// DTLS_ATTRS
	dtlsAttrs = []string{"setup", "fingerprint", "rtcp", "ssrc"}
	// DTLS_ATTRS_RM = DTLS_ATTRS without ssrc
	dtlsAttrsRM = []string{"setup", "fingerprint", "rtcp"}
	// ICE_ATTRS
	iceAttrs = []string{"ice-ufrag", "ice-pwd"}
	// ICE_ATTRS_RM (deduplicated)
	iceAttrsRM = []string{"ice-ufrag", "ice-pwd", "candidate", "ice-options",
		"rtcp-mux", "ice-lite", "end-of-candidates", "extmap", "extmap-allow-mixed", "msid",
		"msid-semantic", "mid", "group", "rtcp-fb"}
	// mod_2_i_mod
	iModDtlsGw  = "M4:1"
	iModIceLite = "M5:1"
)

// sdpAttrName returns the a-header name, i.e. the part before the first ':'.
func sdpAttrName(hdr string) string {
	if i := strings.Index(hdr, ":"); i >= 0 {
		return hdr[:i]
	}
	return hdr
}

// sdpAttrsRm removes a-headers with exact names from the session level (sdp)
// and the media section level (sect), mirroring Python _sdp_attrs_rm.
// Exact-name matching is deliberate: a prefix match would, e.g. remove
// a=rtcp-mux when only a=rtcp is to be removed.
func sdpAttrsRm(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription, names []string) {
	rm := func(hdrs []string) []string {
		keep := make([]string, 0, len(hdrs))
		for _, hdr := range hdrs {
			name := sdpAttrName(hdr)
			if containsStr(names, name) {
				continue
			}
			keep = append(keep, hdr)
		}
		return keep
	}
	if sect != nil {
		sect.SetAHeaders(rm(sect.GetAHeaders()))
	}
	if sdp != nil {
		sdp.SetAHeaders(rm(sdp.GetAHeaders()))
	}
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// splitWS splits s on runs of whitespace into at most n fields
// (Python str.split(None, n) semantics).
func splitWS(s string, n int) []string {
	f := strings.Fields(s)
	if n <= 0 || len(f) <= n {
		return f
	}
	rest := s
	out := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		rest = strings.TrimLeft(rest, " \t\r\n")
		j := 0
		for j < len(rest) && rest[j] != ' ' && rest[j] != '\t' && rest[j] != '\r' && rest[j] != '\n' {
			j++
		}
		out = append(out, rest[:j])
		rest = rest[j:]
	}
	out = append(out, strings.TrimLeft(rest, " \t\r\n"))
	return out
}

// unquote implements Python urllib.parse.unquote semantics: valid %XX pairs
// are decoded, invalid sequences are left as literals, '+' is NOT a space.
func unquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) {
			hi, lo := hexVal(s[i+1]), hexVal(s[i+2])
			if hi < 16 && lo < 16 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 16
}

// NewSubcommand returns the base subcommand with a single command string.
// (Python: subcommand.__init__)
func NewSubcommand(cmd string) *Subcommand {
	return &Subcommand{
		Name:     "subcommand",
		Commands: []string{cmd},
	}
}

// HandleResults is the base strict check: one result per command.
// (Python: subcommand.handle_results)
func (self *Subcommand) HandleResults(results []string, ur *UpdateResult) sippy_types.SipHandlingError {
	if len(results) != len(self.Commands) {
		return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: %s: results=%v", self.Name, results))
	}
	return nil
}

// SubcommandDtls: new DTLS gateway session (Python: subcommand_dtls).
type SubcommandDtls struct {
	*Subcommand
	DtlsMode        string
	DtlsFingerprint string
}

// NewSubcommandDtls creates the "M4:1 S" subcommand.
func NewSubcommandDtls() *SubcommandDtls {
	return &SubcommandDtls{Subcommand: NewSubcommand(iModDtlsGw + " S")}
}

// HandleResults parses "<mode> <fingerprint>" (Python: subcommand_dtls.handle_results).
func (self *SubcommandDtls) HandleResults(results []string, ur *UpdateResult) sippy_types.SipHandlingError {
	if ex := self.Subcommand.HandleResults(results, ur); ex != nil {
		return ex
	}
	dtlsRes := splitWS(results[0], 2)
	if len(dtlsRes) != 2 {
		return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: DTLS: results=%v", results))
	}
	self.DtlsMode, self.DtlsFingerprint = dtlsRes[0], dtlsRes[1]
	ur.SdpSectFins = append(ur.SdpSectFins, self.SdpSectFin)
	return nil
}

// SdpSectFin removes stale DTLS attrs and adds setup/fingerprint
// (Python: subcommand_dtls.sdp_sect_fin).
func (self *SubcommandDtls) SdpSectFin(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) {
	sdpAttrsRm(sdp, sect, dtlsAttrsRM)
	sect.AddHeader("a", "setup:"+self.DtlsMode)
	sect.AddHeader("a", "fingerprint:"+self.DtlsFingerprint)
}

// SubcommandDeDtls: de-DTLS of an existing DTLS gateway leg
// (Python: subcommand_dedtls).
type SubcommandDeDtls struct {
	*Subcommand
}

// NewSubcommandDeDtls builds "M4:1 A|P <fingerprint> [ssrc]" from the SDP
// attrs (first match wins: section level, then session level).
func NewSubcommandDeDtls(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) (*SubcommandDeDtls, error) {
	adict := map[string]string{}
	for _, hdr := range sect.GetAHeaders() {
		if name := sdpAttrName(hdr); containsStr(dtlsAttrs, name) {
			if _, ok := adict[name]; !ok {
				adict[name] = hdrValue(hdr)
			}
		}
	}
	for _, hdr := range sdp.GetAHeaders() {
		if name := sdpAttrName(hdr); containsStr(dtlsAttrs, name) {
			if _, ok := adict[name]; !ok {
				adict[name] = hdrValue(hdr)
			}
		}
	}
	if _, ok := adict["setup"]; !ok {
		return nil, sippy_exceptions.NewSdpParseError("Missing DTLS connection mode parameter")
	}
	if _, ok := adict["fingerprint"]; !ok {
		return nil, sippy_exceptions.NewSdpParseError("Missing DTLS fingerprint parameter")
	}
	asetup := adict["setup"]
	var sDtls string
	switch {
	case asetup == "active" || asetup == "actpass":
		sDtls = iModDtlsGw + " A"
	case asetup == "passive":
		sDtls = iModDtlsGw + " P"
	default:
		return nil, sippy_exceptions.NewSdpParseError(fmt.Sprintf("Unknown connection mode: \"%s\"", asetup))
	}
	sDtls += " " + adict["fingerprint"]
	if ssrc := strings.Fields(adict["ssrc"]); len(ssrc) > 0 {
		sDtls += " " + ssrc[0]
	}
	self := &SubcommandDeDtls{Subcommand: NewSubcommand(sDtls)}
	self.Name = "deDTLS"
	return self, nil
}

func hdrValue(hdr string) string {
	if i := strings.Index(hdr, ":"); i >= 0 {
		return hdr[i+1:]
	}
	return hdr
}

// HandleResults: every result must be "0" (Python: subcommand_dedtls.handle_results).
func (self *SubcommandDeDtls) HandleResults(results []string, ur *UpdateResult) sippy_types.SipHandlingError {
	if ex := self.Subcommand.HandleResults(results, ur); ex != nil {
		return ex
	}
	for _, r := range results {
		if r != "0" {
			return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: %s: results=%v", self.Name, results))
		}
	}
	ur.SdpSectFins = append(ur.SdpSectFins, self.SdpSectFin)
	return nil
}

// SdpSectFin removes DTLS attrs and restores plain RTP transport
// (Python: subcommand_dedtls.sdp_sect_fin).
func (self *SubcommandDeDtls) SdpSectFin(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) {
	sdpAttrsRm(sdp, sect, dtlsAttrsRM)
	sect.GetMHeader().SetTransport("RTP/AVP")
}

// SubcommandIce: new ICE-lite session (Python: subcommand_ice).
type SubcommandIce struct {
	*Subcommand
	IceLufrag     string
	IceLpwd       string
	IceCandidates []string
}

// NewSubcommandIce creates the "M5:1 S" subcommand.
func NewSubcommandIce() *SubcommandIce {
	return &SubcommandIce{Subcommand: NewSubcommand(iModIceLite + " S")}
}

// HandleResults parses "<ufrag> <pwd> <cand1> ..." where each candidate
// token is percent-encoded with a 2-char prefix (Python: subcommand_ice.handle_results).
func (self *SubcommandIce) HandleResults(results []string, ur *UpdateResult) sippy_types.SipHandlingError {
	if ex := self.Subcommand.HandleResults(results, ur); ex != nil {
		return ex
	}
	iceRes := strings.Fields(results[0])
	if len(iceRes) < 3 {
		return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: %s: results=%v", self.Name, results))
	}
	self.IceLufrag, self.IceLpwd = iceRes[0], iceRes[1]
	self.IceCandidates = make([]string, 0, len(iceRes)-2)
	for _, r := range iceRes[2:] {
		if len(r) < 2 {
			return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: %s: results=%v", self.Name, results))
		}
		self.IceCandidates = append(self.IceCandidates, unquote(r[2:]))
	}
	ur.SdpSectFins = append(ur.SdpSectFins, self.SdpSectFin)
	return nil
}

// SdpSectFin rewrites the section as an ICE-lite DTLS media
// (Python: subcommand_ice.sdp_sect_fin).
func (self *SubcommandIce) SdpSectFin(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) {
	sdpAttrsRm(sdp, sect, iceAttrsRM)
	sect.AddHeader("a", "rtcp-mux")
	// RFC 8839 section 5.3: ice-lite is session-level only.
	sdp.AppendAHeader("ice-lite")
	sect.AddHeader("a", "ice-ufrag:"+self.IceLufrag)
	sect.AddHeader("a", "ice-pwd:"+self.IceLpwd)
	for _, cand := range self.IceCandidates {
		sect.AddHeader("a", "candidate:"+cand)
	}
	sect.GetMHeader().SetTransport("UDP/TLS/RTP/SAVP")
}

// SubcommandDeIce: de-ICE of an existing ICE-lite leg (Python: subcommand_deice).
type SubcommandDeIce struct {
	*Subcommand
}

// NewSubcommandDeIce builds "M5:1 A <ufrag> <pwd>" plus "M5:1 C <candidate>"
// per valid UDP candidate. Candidates are filtered: only UDP, and no
// carrier-grade-NAT 192.x addresses outside 192.168.
func NewSubcommandDeIce(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) (*SubcommandDeIce, error) {
	adict := map[string]string{}
	for _, hdr := range sect.GetAHeaders() {
		if name := sdpAttrName(hdr); containsStr(iceAttrs, name) {
			adict[name] = hdrValue(hdr)
		}
	}
	for _, rattr := range iceAttrs {
		if _, ok := adict[rattr]; !ok {
			return nil, sippy_exceptions.NewSdpParseError("Missing ICE " + rattr + " parameter")
		}
	}
	self := &SubcommandDeIce{
		Subcommand: NewSubcommand(iModIceLite + " A " + adict["ice-ufrag"] + " " + adict["ice-pwd"]),
	}
	self.Name = "deICE"
	for _, hdr := range sect.GetAHeaders() {
		if sdpAttrName(hdr) != "candidate" {
			continue
		}
		cand := hdrValue(hdr)
		pts := splitWS(cand, 5)
		if len(pts) < 5 {
			// malformed candidate: skip (Python would raise IndexError)
			continue
		}
		if strings.ToLower(pts[2]) != "udp" {
			continue
		}
		if strings.HasPrefix(pts[4], "192.") && !strings.HasPrefix(pts[4], "192.168.") {
			// Carrier-grade NAT garbage
			continue
		}
		self.Commands = append(self.Commands, iModIceLite+" C "+cand)
	}
	return self, nil
}

// HandleResults: every result must be "0" (Python: subcommand_deice.handle_results).
func (self *SubcommandDeIce) HandleResults(results []string, ur *UpdateResult) sippy_types.SipHandlingError {
	if ex := self.Subcommand.HandleResults(results, ur); ex != nil {
		return ex
	}
	for _, r := range results {
		if r != "0" {
			return sippy_exceptions.NewRtpProxyError(fmt.Sprintf("RTPProxy errored: %s: results=%v", self.Name, results))
		}
	}
	ur.SdpSectFins = append(ur.SdpSectFins, self.SdpSectFin)
	return nil
}

// SdpSectFin removes ICE attrs and msid-bearing ssrc lines
// (Python: subcommand_deice.sdp_sect_fin).
func (self *SubcommandDeIce) SdpSectFin(sdp sippy_types.Sdp, sect *sippy_sdp.SdpMediaDescription) {
	sdpAttrsRm(sdp, sect, iceAttrsRM)
	keep := make([]string, 0, len(sect.GetAHeaders()))
	for _, hdr := range sect.GetAHeaders() {
		if sdpAttrName(hdr) == "ssrc" {
			if parts := splitWS(hdrValue(hdr), 2); len(parts) == 2 &&
				strings.HasPrefix(strings.ToLower(parts[1]), "msid:") {
				continue
			}
		}
		keep = append(keep, hdr)
	}
	sect.SetAHeaders(keep)
}
