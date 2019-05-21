//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
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
package sippy_cli

import (
    "net"

    "sippy/log"
)

type Cli_server_tcp struct {
    Cli_server_stream
    accept_list     []string
}

func NewCli_server_tcp(command_cb func(string, net.Conn) string, address string, logger sippy_log.ErrorLogger) (*Cli_server_tcp, error) {

    tcpaddr, err := net.ResolveTCPAddr("tcp", address)
    if err != nil {
        return nil, err
    }

    listener, err := net.ListenTCP("tcp", tcpaddr)
    if err != nil {
        return nil, err
    }

    self := &Cli_server_tcp{
        Cli_server_stream : Cli_server_stream{
            command_cb  : command_cb,
            listener    : listener,
            logger      : logger,
        },
        accept_list     : nil,
    }
    self.check_acl_cb = self.check_acl
    return self, nil
}

func (self *Cli_server_tcp) check_acl(conn net.Conn) bool {
    if self.accept_list == nil {
        return true
    }
    raddr := net.ParseIP(conn.RemoteAddr().String())
    if raddr != nil {
        for _, addr := range self.accept_list {
            if raddr.String() == addr {
                return true
            }
        }
    }
    return false
}

func (self *Cli_server_tcp) SetAcceptList(acl []string) {
    self.accept_list = acl
}

func (self *Cli_server_tcp) GetAcceptList() []string {
    return self.accept_list
}

func (self *Cli_server_tcp) AcceptListAdd(ip string) {
    self.accept_list = append(self.accept_list, ip)
}
