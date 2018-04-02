// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2017 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2017 Andrii Pylypenko. All rights reserved.
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
    "sort"

    "sippy/headers"
    "sippy/conf"
    "sippy/time"
    "sippy/types"
)
type CCEventRedirect struct {
    CCEventGeneric
    redirect_addresses   []*sippy_header.SipAddress
    scode           int
    scode_reason    string
    body            sippy_types.MsgBody
    config          sippy_conf.Config
}

func NewCCEventRedirect(scode int, scode_reason string, body sippy_types.MsgBody, addrs []*sippy_header.SipAddress, rtime *sippy_time.MonoTime, origin string, config sippy_conf.Config, extra_headers ...sippy_header.SipHeader) *CCEventRedirect {
    return &CCEventRedirect{
        CCEventGeneric  : newCCEventGeneric(rtime, origin, extra_headers...),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : body,
        redirect_addresses : addrs,
        config          : config,
    }
}

func (self *CCEventRedirect) String() string { return "CCEventRedirect" }

func (self *CCEventRedirect) GetRedirectURL() *sippy_header.SipAddress {
    return self.redirect_addresses[0]
}

func (self *CCEventRedirect) GetRedirectURLs() []*sippy_header.SipAddress {
    return self.redirect_addresses
}

func (self *CCEventRedirect) GetContacts() []*sippy_header.SipContact {
    addrs := self.redirect_addresses
    if addrs == nil || len(addrs) == 0 {
        return nil
    }
    ret := make([]*sippy_header.SipContact, len(addrs))
    for i, addr := range addrs {
        ret[i] = sippy_header.NewSipContactFromAddress(addr, self.config)
    }
    return ret
}

func (self *CCEventRedirect) SortAddresses() {
    if len(self.redirect_addresses) == 1 {
        return
    }
    sort.Sort(sortRedirectAddresses(self.redirect_addresses))
}

type sortRedirectAddresses []*sippy_header.SipAddress
func (self sortRedirectAddresses) Len() int { return len(self) }
func (self sortRedirectAddresses) Swap(x, y int) { self[x], self[y] = self[y], self[x] }
func (self sortRedirectAddresses) Less(x, y int) bool {
    // descending order
    return self[x].GetQ() > self[y].GetQ()
}
