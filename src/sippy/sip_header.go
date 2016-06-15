// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
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
package sippy

import (
    "fmt"
    "strings"
    "sippy/headers"
)

var sip_header_name_map = map[string]func(body string) ([]sippy_header.SipHeader, error) {
    "cseq"              : sippy_header.ParseSipCSeq,
    "call-id"           : sippy_header.ParseSipCallId,
    "i"                 : sippy_header.ParseSipCallId,
    "from"              : sippy_header.ParseSipFrom,
    "f"                 : sippy_header.ParseSipFrom,
    "to"                : sippy_header.ParseSipTo,
    "t"                 : sippy_header.ParseSipTo,
    "max-forwards"      : sippy_header.ParseSipMaxForwards,
    "via"               : sippy_header.ParseSipVia,
    "v"                 : sippy_header.ParseSipVia,
    "content-length"    : sippy_header.ParseSipContentLength,
    "l"                 : sippy_header.ParseSipContentLength,
    "content-type"      : sippy_header.ParseSipContentType,
    "c"                 : sippy_header.ParseSipContentType,
    "expires"           : sippy_header.ParseSipExpires,
    "record-route"      : sippy_header.ParseSipRecordRoute,
    "route"             : sippy_header.ParseSipRoute,
    "contact"           : sippy_header.ParseSipContact,
    "m"                 : sippy_header.ParseSipContact,
    "www-authenticate"  : sippy_header.ParseSipWWWAuthenticate,
    "authorization"     : sippy_header.ParseSipAuthorization,
    "server"            : sippy_header.ParseSipServer,
    "user-agent"        : sippy_header.ParseSipUserAgent,
    "cisco-guid"        : sippy_header.ParseSipCiscoGUID,
    "h323-conf-id"      : sippy_header.ParseSipH323ConfId,
    "also"              : sippy_header.ParseSipAlso,
    "refer-to"          : sippy_header.ParseSipReferTo,
    "r"                 : sippy_header.ParseSipReferTo,
    "cc-diversion"      : sippy_header.ParseSipCCDiversion,
    "referred-by"       : sippy_header.ParseSipReferredBy,
    "proxy-authenticate": sippy_header.ParseSipProxyAuthenticate,
    "proxy-authorization": sippy_header.ParseSipProxyAuthorization,
    "replaces"          : sippy_header.ParseSipReplaces,
    "reason"            : sippy_header.ParseSipReason,
    "warning"           : sippy_header.ParseSipWarning,
}

func ParseSipHeader(s string) ([]sippy_header.SipHeader, error) {
    res := strings.SplitN(s, ":", 2)
    if len(res) != 2 {
        return nil, fmt.Errorf("Bad header line: '%s'", s)
    }
    name := strings.TrimSpace(res[0])
    body := strings.TrimSpace(res[1])
    factory, ok := sip_header_name_map[strings.ToLower(name)]
    if ok {
        return factory(body)
    }
    return []sippy_header.SipHeader{ sippy_header.ParseSipGenericHF(name, body) }, nil
}
