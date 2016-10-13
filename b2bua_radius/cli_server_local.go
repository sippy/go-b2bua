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
package main

import (
    "bufio"
    "net"
    "os"

    "sippy/log"
    "sippy/utils"
)

type Cli_server_local struct {
    command_cb      func(string) string
    listener        net.Listener
    logger          sippy_log.ErrorLogger
}
/*
from twisted.internet.protocol import Factory
from twisted.internet import reactor
from Cli_session import Cli_session
from os import chown, unlink
from os.path import exists

class Cli_server_local(Factory):
    command_cb = nil
*/
func NewCli_server_local(command_cb func(string) string, address string, logger sippy_log.ErrorLogger/*, sock_owner = nil*/) (*Cli_server_local, error) {
    if _, err := os.Stat(address); err == nil {
        err = os.Remove(address)
        if err != nil { return nil, err }
    }
    addr, err := net.ResolveUnixAddr("unix", address)
    if err != nil { return nil, err }

    listener, err := net.ListenUnix("unix", addr)
    if err != nil { return nil, err }

    self := &Cli_server_local{
        command_cb  : command_cb,
//        protocol    : NewCli_session,
        listener    : listener,
        logger      : logger,
    }
    //if address == nil:
    //    address = '/var/run/ccm.sock'
    //if sock_owner != nil:
    //    chown(address, sock_owner[0], sock_owner[1])
    return self, nil
}

func (self *Cli_server_local) Start() {
    go self.run()
}

func (self *Cli_server_local) run() {
    for {
        conn, err := self.listener.Accept()
        if err != nil {
            break
        }
        go sippy_utils.SafeCall(func() { self.handle_request(conn) }, nil, self.logger)
    }
}

func (self *Cli_server_local) handle_request(conn net.Conn) {
    defer conn.Close()
    reader := bufio.NewReader(conn)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            break
        }
        res := self.command_cb(line)
        _, err = conn.Write([]byte(res))
        if err != nil {
            break
        }
    }
}
/*
    def buildProtocol(self, addr):
        p = Factory.buildProtocol(self, addr)
        p.command_cb = self.command_cb
        return p

if __name__ == '__main__':
    def callback(clm, cmd):
        print cmd
        return False
    f = Cli_server_local(callback)
    reactor.run()
*/
