// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006 Sippy Software, Inc. All rights reserved.
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
    "bufio"
    "fmt"
    "net"
    "os"
    "sync"
    "syscall"

    "sippy/log"
)

type CLIManagerIface interface {
    Close()
    Send(string)
    RemoteAddr() net.Addr
}

type CLIConnectionManager struct {
    tcp         bool
    sock        net.Listener
    command_cb  func(clim CLIManagerIface, cmd string)
    accept_list map[string]bool
    accept_list_lock sync.RWMutex
    logger      sippy_log.ErrorLogger
}

func NewCLIConnectionManagerUnix(command_cb func(clim CLIManagerIface, cmd string), address string, uid, gid int, logger sippy_log.ErrorLogger) (*CLIConnectionManager, error) {
    addr, err := net.ResolveUnixAddr("unix", address)
    if err != nil {
        return nil, err
    }
    conn, err := net.DialUnix("unix", nil, addr)
    if err == nil {
        conn.Close()
        return nil, fmt.Errorf("Another process listens on %s", address)
    }
    os.Remove(address)
    sock, err := net.ListenUnix("unix", addr)
    if err != nil {
        return nil, err
    }
    os.Chown(address, uid, gid)
    os.Chmod(address, 0660)
    return &CLIConnectionManager{
        command_cb  : command_cb,
        sock        : sock,
        tcp         : false,
        logger      : logger,
    }, nil
}

func NewCLIConnectionManagerTcp(command_cb func(clim CLIManagerIface, cmd string), address string, logger sippy_log.ErrorLogger) (*CLIConnectionManager, error) {
    addr, err := net.ResolveTCPAddr("tcp", address)
    if err != nil {
        return nil, err
    }
    sock, err := net.ListenTCP("tcp", addr)
    if err != nil {
        return nil, err
    }
    return &CLIConnectionManager{
        command_cb  : command_cb,
        sock        : sock,
        tcp         : true,
        logger      : logger,
    }, nil
}

func (self *CLIConnectionManager) Start() {
    go self.run()
}

func (self *CLIConnectionManager) run() {
    defer self.sock.Close()
    for {
        conn, err := self.sock.Accept()
        if err != nil {
            self.logger.Error(err.Error())
            break
        }
        go self.handle_accept(conn)
    }
}

func (self CLIConnectionManager) handle_accept(conn net.Conn) {
    if self.tcp {
        raddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
        if err != nil {
            self.logger.Error("SplitHostPort failed. Possible bug: " + err.Error())
            // Not reached
            conn.Close()
            return
        }
        self.accept_list_lock.RLock()
        defer self.accept_list_lock.RUnlock()
        if self.accept_list != nil {
            if _, ok := self.accept_list[raddr]; ! ok {
                conn.Close()
                return
            }
        }
    }
    cm := NewCLIManager(conn, self.command_cb)
    go cm.run()
}

func (self *CLIConnectionManager) Shutdown() {
    self.sock.Close()
}

func (self *CLIConnectionManager) GetAcceptList() []string {
    self.accept_list_lock.RLock()
    defer self.accept_list_lock.RUnlock()
    if self.accept_list != nil {
        ret := make([]string, 0, len(self.accept_list))
        for addr, _ := range self.accept_list {
            ret = append(ret, addr)
        }
        return ret
    }
    return nil
}

func (self *CLIConnectionManager) SetAcceptList(acl []string) {
    accept_list := make(map[string]bool)
    for _, addr := range acl {
        accept_list[addr] = true
    }
    self.accept_list_lock.Lock()
    self.accept_list = accept_list
    self.accept_list_lock.Unlock()
}

func (self *CLIConnectionManager) AcceptListAppend(ip string) {
    self.accept_list_lock.Lock()
    if self.accept_list == nil {
        self.accept_list = make(map[string]bool)
    }
    self.accept_list[ip] = true
    self.accept_list_lock.Unlock()
}

func (self *CLIConnectionManager) AcceptListRemove(ip string) {
    self.accept_list_lock.Lock()
    if self.accept_list != nil {
        delete(self.accept_list, ip)
    }
    self.accept_list_lock.Unlock()

}

type CLIManager struct {
    sock        net.Conn
    command_cb  func(CLIManagerIface, string)
    wbuffer     string
}

func NewCLIManager(sock net.Conn, command_cb func(CLIManagerIface, string)) *CLIManager {
    return &CLIManager{
        sock        : sock,
        command_cb  : command_cb,
    }
}

func (self *CLIManager) run() {
    defer self.sock.Close()
    reader := bufio.NewReader(self.sock)
    for {
        line, _, err := reader.ReadLine()
        if err != nil && err != syscall.EINTR {
            return
        } else {
            self.command_cb(self, string(line))
        }
        for self.wbuffer != "" {
            n, err := self.sock.Write([]byte(self.wbuffer))
            if err != nil && err != syscall.EINTR {
                return
            }
            self.wbuffer = self.wbuffer[n:]
        }
    }
}

func (self *CLIManager) Send(data string) {
    self.wbuffer += data
}

func (self *CLIManager) Close() {
    self.sock.Close()
}

func (self *CLIManager) RemoteAddr() net.Addr {
    return self.sock.RemoteAddr()
}
