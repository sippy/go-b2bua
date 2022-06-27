// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
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
    "errors"
    "strconv"
    "strings"
    "sync"
    "unicode"

    "sippy/utils"
)

func extract_to_next_token(s string, match string, invert bool) (string, string) {
    i := 0
    for i < len(s) {
        if (! invert && strings.IndexByte(match, s[i]) == -1) || (invert && strings.IndexByte(match, s[i]) != -1) {
            break
        }
        i++
    }
    if i == 0 {
        return "", s
    }
    if i == len(s) {
        return s, ""
    }
    return s[:i], s[i:]
}

type UpdateLookupOpts struct {
    DestinationIP   string
    LocalIP         string
    Codecs          []string
    Otherparams     string
    RemoteIP        string
    RemotePort      string
    FromTag         string
    ToTag           string
    NotifySocket    string
    NotifyTag       string
}

func NewUpdateLookupOpts(s, args string) (*UpdateLookupOpts, error) {
    arr := sippy_utils.FieldsN(args, 3)
    if len(arr) != 3 {
        return nil, errors.New("The lookup opts must have at least three arguments")
    }
    self := &UpdateLookupOpts{
        RemoteIP        : arr[0],
        RemotePort      : arr[1],
    }
    arr = sippy_utils.FieldsN(arr[2], 2)
    self.FromTag = arr[0]
    if len(arr) > 1 {
        arr2 := sippy_utils.FieldsN(arr[1], 3)
        switch len(arr2) {
        case 1:
            self.ToTag = arr2[0]
        case 2:
            self.NotifySocket, self.NotifyTag = arr2[0], arr2[1]
        default:
            self.ToTag, self.NotifySocket, self.NotifyTag = arr2[0], arr2[1], arr2[2]
        }
    }
    for len(s) > 0 {
        var val string
        if s[0] == 'R' {
            val, s = extract_to_next_token(s[1:], "1234567890.", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.DestinationIP = val
            }
        }
        switch s[0] {
        case 'L':
            val, s = extract_to_next_token(s[1:], "1234567890.", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.LocalIP = val
            }
        case 'c':
            val, s = extract_to_next_token(s[1:], "1234567890,", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.Codecs = strings.Split(val, ",")
            }
        default:
            val, s = extract_to_next_token(s, "cR", true)
            if len(val) > 0 {
                self.Otherparams += val
            }
        }
    }
    return self, nil
}

func (self *UpdateLookupOpts) Getstr(call_id string/*, swaptags bool*/) string {
    s := ""
    if self.DestinationIP != "" {
        s += "R" + self.DestinationIP
    }
    if self.LocalIP != "" {
        s += "L" + self.LocalIP
    }
    if self.Codecs != nil {
        s += "c" + strings.Join(self.Codecs, ",")
    }
    s += self.Otherparams
    s += " " + call_id
    if self.RemoteIP != "" {
        s += " " + self.RemoteIP
    }
    if self.RemotePort != "" {
        s += " " + self.RemotePort
    }
    /*
    from_tag, to_tag := self.FromTag, self.to_tag
    if swaptags {
        if self.to_tag == "" {
            return "", errors.New('UpdateLookupOpts::Getstr(swaptags = True): to_tag is not set')
        }
        to_tag, from_tag = self.FromTag, self.to_tag
    }
    */
    if self.FromTag != "" {
        s += " " + self.FromTag
    }
    if self.ToTag != "" {
        s += " " + self.ToTag
    }
    if self.NotifySocket != "" {
        s += " " + self.NotifySocket
    }
    if self.NotifyTag != "" {
        s += " " + self.NotifyTag
    }
    return s
}

type Rtp_proxy_cmd struct {
    Type            byte
    ULOpts          *UpdateLookupOpts
    CommandOpts     string
    CallId          string
    Args            string
    Nretr           int
}

func NewRtp_proxy_cmd(cmd string) (*Rtp_proxy_cmd, error) {
    self := &Rtp_proxy_cmd{
        Type    : strings.ToUpper(cmd[:1])[0],
    }
    switch self.Type {
    case 'U': fallthrough
    case 'L': fallthrough
    case 'D': fallthrough
    case 'P': fallthrough
    case 'S': fallthrough
    case 'R': fallthrough
    case 'C': fallthrough
    case 'Q':
        var command_opts, args string
        arr := sippy_utils.FieldsN(cmd, 3)
        if len(arr) != 3 {
            return nil, errors.New("The command must have at least three parts")
        }
        command_opts, self.CallId, args = arr[0], arr[1], arr[2]
        switch self.Type {
        case 'U': fallthrough
        case 'L':
            var err error
            self.ULOpts, err = NewUpdateLookupOpts(command_opts[1:], args)
            if err != nil {
                return nil, err
            }
        default:
            self.Args = args
            self.CommandOpts = command_opts[1:]
        }
    case 'G':
        if ! unicode.IsSpace([]rune(cmd)[1]) {
            cparts := sippy_utils.FieldsN(cmd[1:], 2)
            if len(cparts) > 1 {
                self.CommandOpts, self.Args = cparts[0], cparts[1]
            } else {
                self.CommandOpts = cparts[0]
            }
        } else {
            self.Args = strings.TrimSpace(cmd[1:])
        }
    default:
        self.CommandOpts = cmd[1:]
    }
    return self, nil
}

func (self *Rtp_proxy_cmd) String() string {
    s := string([]byte{ self.Type })
    if self.ULOpts != nil {
        s += self.ULOpts.Getstr(self.CallId)
    } else {
        if self.CommandOpts != "" {
            s += self.CommandOpts
        }
        if self.CallId != "" {
            s += " " + self.CallId
        }
    }
    if self.Args != "" {
        s += " " + self.Args
    }
    return s
}

type Rtpp_stats struct {
    spookyprefix    string
    all_names       []string
    Verbose         bool
    dict            map[string]int64
    dict_lock       sync.Mutex
    total_duration  float64
}

func NewRtpp_stats(snames []string) *Rtpp_stats {
    self := &Rtpp_stats{
        Verbose         : false,
        spookyprefix    : "",
        dict            : make(map[string]int64),
        all_names       : snames,
    }
    for _, sname := range snames {
        if sname != "total_duration" {
            self.dict[self.spookyprefix + sname] = 0
        }
    }
    return self
}

func (self *Rtpp_stats) AllNames() []string {
    return self.all_names
}
/*
    def __iadd__(self, other):
        for sname in self.all_names:
            aname = self.spookyprefix + sname
            self.__dict__[aname] += other.__dict__[aname]
        return self
*/
func (self *Rtpp_stats) ParseAndAdd(rstr string) error {
    rparts := sippy_utils.FieldsN(rstr, len(self.all_names))
    for i, name := range self.all_names {
        if name == "total_duration" {
            rval, err := strconv.ParseFloat(rparts[i], 64)
            if err != nil {
                return err
            }
            self.total_duration += rval
        } else {
            rval, err := strconv.ParseInt(rparts[i], 10, 64)
            if err != nil {
                return err
            }
            aname := self.spookyprefix + self.all_names[i]
            self.dict_lock.Lock()
            self.dict[aname] += rval
            self.dict_lock.Unlock()
        }
    }
    return nil
}

func (self *Rtpp_stats) String() string {
    rvals := make([]string, 0, len(self.all_names))
    for _, sname := range self.all_names {
        var rval string

        if sname == "total_duration" {
            rval = strconv.FormatFloat(self.total_duration, 'f', -1, 64)
        } else {
            aname := self.spookyprefix + sname
            self.dict_lock.Lock()
            rval = strconv.FormatInt(self.dict[aname], 10)
            self.dict_lock.Unlock()
        }
        if self.Verbose {
            rval = sname + "=" + rval
        }
        rvals = append(rvals, rval)
    }
    return strings.Join(rvals, " ")
}
/*
if __name__ == '__main__':
    rc = Rtp_proxy_cmd('G nsess_created total_duration')
    print(rc)
    print(rc.args)
    print(rc.command_opts)
    rc = Rtp_proxy_cmd('Gv nsess_created total_duration')
    print(rc)
    print(rc.args)
    print(rc.command_opts)
*/
