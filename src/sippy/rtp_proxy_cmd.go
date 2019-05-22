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
    destination_ip  string
    local_ip        string
    codecs          []string
    otherparams     string
    remote_ip       string
    remote_port     string
    from_tag        string
    to_tag          string
    notify_socket   string
    notify_tag      string
}

func NewUpdateLookupOpts(s, args string) (*UpdateLookupOpts, error) {
    arr := sippy_utils.FieldsN(args, 3)
    if len(arr) != 3 {
        return nil, errors.New("The lookup opts must have at least three arguments")
    }
    self := &UpdateLookupOpts{
        remote_ip       : arr[0],
        remote_port     : arr[1],
    }
    arr = sippy_utils.FieldsN(arr[2], 2)
    self.from_tag = arr[0]
    if len(arr) > 1 {
        arr2 := sippy_utils.FieldsN(arr[1], 3)
        switch len(arr2) {
        case 1:
            self.to_tag = arr2[0]
        case 2:
            self.notify_socket, self.notify_tag = arr2[0], arr2[1]
        default:
            self.to_tag, self.notify_socket, self.notify_tag = arr2[0], arr2[1], arr2[2]
        }
    }
    for len(s) > 0 {
        var val string
        if s[0] == 'R' {
            val, s = extract_to_next_token(s[1:], "1234567890.", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.destination_ip = val
            }
        }
        switch s[0] {
        case 'L':
            val, s = extract_to_next_token(s[1:], "1234567890.", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.local_ip = val
            }
        case 'c':
            val, s = extract_to_next_token(s[1:], "1234567890,", false)
            val = strings.TrimSpace(val)
            if len(val) > 0 {
                self.codecs = strings.Split(val, ",")
            }
        default:
            val, s = extract_to_next_token(s, "cR", true)
            if len(val) > 0 {
                self.otherparams += val
            }
        }
    }
    return self, nil
}

func (self *UpdateLookupOpts) getstr(call_id string/*, swaptags bool*/) string {
    s := ""
    if self.destination_ip != "" {
        s += "R" + self.destination_ip
    }
    if self.local_ip != "" {
        s += "L" + self.local_ip
    }
    if self.codecs != nil {
        s += "c" + strings.Join(self.codecs, ",")
    }
    s += self.otherparams
    s += " " + call_id
    if self.remote_ip != "" {
        s += " " + self.remote_ip
    }
    if self.remote_port != "" {
        s += " " + self.remote_port
    }
    /*
    from_tag, to_tag := self.from_tag, self.to_tag
    if swaptags {
        if self.to_tag == "" {
            return "", errors.New('UpdateLookupOpts::getstr(swaptags = True): to_tag is not set')
        }
        to_tag, from_tag = self.from_tag, self.to_tag
    }
    */
    if self.from_tag != "" {
        s += " " + self.from_tag
    }
    if self.to_tag != "" {
        s += " " + self.to_tag
    }
    if self.notify_socket != "" {
        s += " " + self.notify_socket
    }
    if self.notify_tag != "" {
        s += " " + self.notify_tag
    }
    return s
}

type Rtp_proxy_cmd struct {
    _type           byte
    ul_opts         *UpdateLookupOpts
    command_opts    string
    call_id         string
    args            string
    //nretr = None
}

func NewRtp_proxy_cmd(cmd string) (*Rtp_proxy_cmd, error) {
    self := &Rtp_proxy_cmd{
        _type       : strings.ToUpper(cmd[:1])[0],
    }
    switch self._type {
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
        command_opts, self.call_id, args = arr[0], arr[1], arr[2]
        switch self._type {
        case 'U': fallthrough
        case 'L':
            var err error
            self.ul_opts, err = NewUpdateLookupOpts(command_opts[1:], args)
            if err != nil {
                return nil, err
            }
        default:
            self.args = args
            self.command_opts = command_opts[1:]
        }
    case 'G':
        if ! unicode.IsSpace([]rune(cmd)[1]) {
            cparts := sippy_utils.FieldsN(cmd[1:], 2)
            if len(cparts) > 1 {
                self.command_opts, self.args = cparts[0], cparts[1]
            } else {
                self.command_opts = cparts[0]
            }
        } else {
            self.args = strings.TrimSpace(cmd[1:])
        }
    default:
        self.command_opts = cmd[1:]
    }
    return self, nil
}

/*
    def __str__(self):
        s = self.type
        if self.ul_opts != None:
            s += self.ul_opts.getstr(self.call_id)
        else:
            if self.command_opts != None:
                s += self.command_opts
            if self.call_id != None:
                s = '%s %s' % (s, self.call_id)
        if self.args != None:
            s = '%s %s' % (s, self.args)
        return s

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
