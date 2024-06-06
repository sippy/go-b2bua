//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2024 Sippy Software, Inc. All rights reserved.
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
    "os"
    "strconv"
    "strings"

    "github.com/sippy/go-b2bua/sippy"
    "github.com/sippy/go-b2bua/sippy/cli"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/utils"
)

func main() {
    global_config := NewMyConfigParser()
    err := global_config.Parse()
    if err != nil {
        println(err.Error())
        return
    }

    var static_route *B2BRoute
    if global_config.Static_route != "" {
        static_route, err = NewB2BRoute(global_config.Static_route, global_config)
        if err != nil {
            println("Error parsing the static route")
            println(err.Error())
            return
        }
    } else if ! global_config.Auth_enable {
        println("ERROR: static route should be specified when Radius auth is disabled")
        return
    }

    if ! global_config.Foreground {
        sippy_utils.Daemonize(global_config.Logfile, -1, -1, global_config.ErrorLogger())
    }
    rtp_proxy_clients := make([]sippy_types.RtpProxyClient, len(global_config.Rtp_proxy_clients))
    for i, address := range global_config.Rtp_proxy_clients_arr {
        opts, err := sippy.NewRtpProxyClientOpts(address, nil /*bind_address*/, global_config, global_config.ErrorLogger())
        if err != nil {
            println("Cannot initialize rtpproxy client: " + err.Error())
            return
        }
        opts.SetHeartbeatInterval(global_config.Hrtb_ival_dur)
        opts.SetHeartbeatRetryInterval(global_config.Hrtb_retr_ival_dur)
        rtpp := sippy.NewRtpProxyClient(opts)
        err = rtpp.Start()
        if err != nil {
            println("Cannot initialize rtpproxy client: " + err.Error())
            return
        }
        rtp_proxy_clients[i] = rtpp
    }

    var radius_client *RadiusClient
    var radius_auth *RadiusAuthorisation

    if global_config.Auth_enable || global_config.Acct_enable {
        radius_client = NewRadiusClient(global_config)
        radius_auth = NewRadiusAuthorisation(radius_client, global_config)
    }
    global_config.SetMyUAName("Sippy B2BUA (RADIUS)")

    cmap := NewCallMap(global_config, rtp_proxy_clients, static_route, radius_client, radius_auth)
/*
    if global_config.getdefault('xmpp_b2bua_id', nil) != nil:
        global_config['_xmpp_mode'] = true
*/
    sip_tm, err := sippy.NewSipTransactionManager(global_config, cmap)
    if err != nil {
        println("Cannot initialize SipTransactionManager: " + err.Error())
        return
    }
    //sip_tm.nat_traversal = global_config.nat_traversal
    cmap.Sip_tm = sip_tm
    if global_config.Sip_proxy != "" {
        var sip_proxy *sippy_net.HostPort
        host_port := strings.SplitN(global_config.Sip_proxy, ":", 2)
        if len(host_port) == 1 {
            sip_proxy = sippy_net.NewHostPort(host_port[0], "5060")
        } else {
            sip_proxy = sippy_net.NewHostPort(host_port[0], host_port[1])
        }
        cmap.Proxy = sippy.NewStatefulProxy(sip_tm, sip_proxy, global_config)
    }

    cmdfile := global_config.B2bua_socket
    if strings.HasPrefix(cmdfile, "unix:") {
        cmdfile = cmdfile[5:]
    }
    cli_server, err := sippy_cli.NewCLIConnectionManagerUnix(cmap.RecvCommand, cmdfile, os.Getuid(), os.Getgid(), global_config.ErrorLogger())
    if err != nil {
        println("Cannot initialize Cli_server: " + err.Error())
        return
    }
    cli_server.Start()

    if ! global_config.Foreground {
        fd, err := os.OpenFile(global_config.Pidfile, os.O_WRONLY | os.O_CREATE, 0644)
        if err != nil {
            global_config.ErrorLogger().Error("Cannot open PID file: " + err.Error())
            return
        }
        fd.WriteString(strconv.Itoa(os.Getpid()) + "\n")
    }
    sip_tm.Run()
}
