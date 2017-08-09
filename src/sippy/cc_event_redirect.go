package sippy

import (
    "sort"

    "sippy/headers"
    "sippy/time"
    "sippy/types"
)
type CCEventRedirect struct {
    CCEventGeneric
    redirect_urls   []*sippy_header.SipURL
    scode           int
    scode_reason    string
    body            sippy_types.MsgBody
}

func NewCCEventRedirect(scode int, scode_reason string, body sippy_types.MsgBody, urls []*sippy_header.SipURL, rtime *sippy_time.MonoTime, origin string, extra_headers ...sippy_header.SipHeader) *CCEventRedirect {
    return &CCEventRedirect{
        CCEventGeneric  : newCCEventGeneric(rtime, origin, extra_headers...),
        scode           : scode,
        scode_reason    : scode_reason,
        body            : body,
        redirect_urls   : urls,
    }
}

func (self *CCEventRedirect) String() string { return "CCEventRedirect" }

func (self *CCEventRedirect) GetRedirectURL() *sippy_header.SipURL {
    return self.redirect_urls[0]
}

func (self *CCEventRedirect) GetRedirectURLs() []*sippy_header.SipURL {
    return self.redirect_urls
}

func (self *CCEventRedirect) GetContacts() []*sippy_header.SipContact {
    urls := self.redirect_urls
    if urls == nil || len(urls) == 0 {
        return nil
    }
    ret := make([]*sippy_header.SipContact, len(urls))
    for i, u := range urls {
        ret[i] = sippy_header.NewSipContactFromAddress(sippy_header.NewSipAddress("", u))
    }
    return ret
}

func (self *CCEventRedirect) SortURLs() {
    if len(self.redirect_urls) == 1 {
        return
    }
    sort.Sort(sortRedirectURLs{ self.redirect_urls })
}

type sortRedirectURLs struct {
    urls        []*sippy_header.SipURL
}

func (self sortRedirectURLs) Len() int {
    return len(self.urls)
}

func (self sortRedirectURLs) Swap(x, y int) {
    self.urls[x], self.urls[y] = self.urls[y], self.urls[x]
}

func (self sortRedirectURLs) Less(x, y int) bool {
    return self.urls[x].Q > self.urls[y].Q // descending order
}
