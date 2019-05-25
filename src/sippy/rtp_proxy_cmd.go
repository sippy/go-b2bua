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
    "strings"
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
    args            string
    //nretr = None
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
            self.args = args
            self.CommandOpts = command_opts[1:]
        }
    case 'G':
        if ! unicode.IsSpace([]rune(cmd)[1]) {
            cparts := sippy_utils.FieldsN(cmd[1:], 2)
            if len(cparts) > 1 {
                self.CommandOpts, self.args = cparts[0], cparts[1]
            } else {
                self.CommandOpts = cparts[0]
            }
        } else {
            self.args = strings.TrimSpace(cmd[1:])
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
    if self.args != "" {
        s += " " + self.args
    }
    return s
}

/*
class Rtpp_stats(object):
    spookyprefix = ''
    verbose = False

    def __init__(self, snames):
        all_types = []
        for sname in snames:
            if sname != 'total_duration':
                stype = int
            else:
                stype = float
            self.__dict__[self.spookyprefix + sname] = stype()
            all_types.append(stype)
        self.all_names = tuple(snames)
        self.all_types = tuple(all_types)

    def __iadd__(self, other):
        for sname in self.all_names:
            aname = self.spookyprefix + sname
            self.__dict__[aname] += other.__dict__[aname]
        return self

    def parseAndAdd(self, rstr):
        rparts = rstr.split(None, len(self.all_names) - 1)
        for i in range(0, len(self.all_names)):
            stype = self.all_types[i]
            rval = stype(rparts[i])
            aname = self.spookyprefix + self.all_names[i]
            self.__dict__[aname] += rval

    def __str__(self):
        aname = self.spookyprefix + self.all_names[0]
        if self.verbose:
            rval = '%s=%s' % (self.all_names[0], str(self.__dict__[aname]))
        else:
            rval = str(self.__dict__[aname])
        for sname in self.all_names[1:]:
            aname = self.spookyprefix + sname
            if self.verbose:
                rval += ' %s=%s' % (sname, str(self.__dict__[aname]))
            else:
                rval += ' %s' % str(self.__dict__[aname])
        return rval

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
