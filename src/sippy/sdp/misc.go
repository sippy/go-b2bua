package sippy_sdp

import (
    "sippy/conf"
)

type SdpHeader interface {
    String() string
    LocalStr(hostport *sippy_conf.HostPort) string
}

type Sdp_header_and_name struct {
    Name    string
    Header  SdpHeader
}
