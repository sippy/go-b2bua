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

var sip_header_name_map = map[string]func(body string) ([]sippy_header.SipHeader) {
    "cseq"              : sippy_header.CreateSipCSeq,
    "rseq"              : sippy_header.CreateSipRSeq,
    "rack"              : sippy_header.CreateSipRAck,
    "call-id"           : sippy_header.CreateSipCallId,
    "i"                 : sippy_header.CreateSipCallId,
    "from"              : sippy_header.CreateSipFrom,
    "f"                 : sippy_header.CreateSipFrom,
    "to"                : sippy_header.CreateSipTo,
    "t"                 : sippy_header.CreateSipTo,
    "max-forwards"      : sippy_header.CreateSipMaxForwards,
    "via"               : sippy_header.CreateSipVia,
    "v"                 : sippy_header.CreateSipVia,
    "content-length"    : sippy_header.CreateSipContentLength,
    "l"                 : sippy_header.CreateSipContentLength,
    "content-type"      : sippy_header.CreateSipContentType,
    "c"                 : sippy_header.CreateSipContentType,
    "expires"           : sippy_header.CreateSipExpires,
    "record-route"      : sippy_header.CreateSipRecordRoute,
    "route"             : sippy_header.CreateSipRoute,
    "contact"           : sippy_header.CreateSipContact,
    "m"                 : sippy_header.CreateSipContact,
    "www-authenticate"  : sippy_header.CreateSipWWWAuthenticate,
    "authorization"     : sippy_header.CreateSipAuthorization,
    "server"            : sippy_header.CreateSipServer,
    "user-agent"        : sippy_header.CreateSipUserAgent,
    "cisco-guid"        : sippy_header.CreateSipCiscoGUID,
    "h323-conf-id"      : sippy_header.CreateSipH323ConfId,
    "also"              : sippy_header.CreateSipAlso,
    "refer-to"          : sippy_header.CreateSipReferTo,
    "r"                 : sippy_header.CreateSipReferTo,
    "cc-diversion"      : sippy_header.CreateSipCCDiversion,
    "referred-by"       : sippy_header.CreateSipReferredBy,
    "proxy-authenticate": sippy_header.CreateSipProxyAuthenticate,
    "proxy-authorization": sippy_header.CreateSipProxyAuthorization,
    "replaces"          : sippy_header.CreateSipReplaces,
    "reason"            : sippy_header.CreateSipReason,
    "warning"           : sippy_header.CreateSipWarning,
    "diversion"         : sippy_header.CreateSipDiversion,
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
        return factory(body), nil
    }
    return []sippy_header.SipHeader{ sippy_header.NewSipGenericHF(strings.Title(name), body) }, nil
}
