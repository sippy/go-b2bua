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

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sippy/go-b2bua/sippy"
	"github.com/sippy/go-b2bua/sippy/conf"
	"github.com/sippy/go-b2bua/sippy/log"
	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/rtp_proxy"
	embedded "github.com/sippy/go-b2bua/sippy/rtp_proxy/embedded"
	rtp_session "github.com/sippy/go-b2bua/sippy/rtp_proxy/session"
	"github.com/sippy/go-b2bua/sippy/types"
)

var next_cc_id chan int64

func init() {
	next_cc_id = make(chan int64)
	go func() {
		var id int64 = 1
		for {
			next_cc_id <- id
			id++
		}
	}()
}

type callController struct {
	destination *sippy_net.HostPort
	uaA         sippy_types.UA
	uaO         sippy_types.UA
	lock        *sync.Mutex // this must be a reference to prevent memory leak
	id          int64
	cmap        *callMap
	media       *rtp_session.Rtp_proxy_session
}

func NewCallController(cmap *callMap, destination *sippy_net.HostPort, callerWebRTC bool) (*callController, error) {
	self := &callController{
		id:   <-next_cc_id,
		uaO:  nil,
		lock: new(sync.Mutex),
		cmap: cmap,
	}
	media, err := rtp_session.NewRtp_proxy_session(cmap.config, []sippy_types.RtpProxyClient{cmap.config.rtpp}, "", "", "", "", "", self.lock)
	if err != nil {
		return nil, err
	}
	self.media = media
	if callerWebRTC {
		self.media.SetCallerWebRTC(true)
	} else {
		self.media.SetCalleeWebRTC(true)
	}
	self.uaA = sippy.NewUA(cmap.sip_tm, cmap.config, cmap.config.nh_addr, self, self.lock, nil)
	self.destination = destination
	self.uaA.SetDeadCb(self.aDead)
	//self.uaA.SetCreditTime(5 * time.Second)
	return self, nil
}

func (self *callController) RecvEvent(event sippy_types.CCEvent, ua sippy_types.UA) {
	if ua == self.uaA {
		if self.uaO == nil {
			if _, ok := event.(*sippy.CCEventTry); !ok {
				// Some weird event received
				self.uaA.RecvEvent(sippy.NewCCEventDisconnect(nil, event.GetRtime(), ""))
				return
			}
			self.uaO = sippy.NewUA(self.cmap.sip_tm, self.cmap.config, self.cmap.config.nh_addr, self, self.lock, nil)
			self.uaO.SetDeadCb(self.oDead)
			self.uaO.SetOnLocalSdpChange(self.media.OnCallerSdpChange)
			self.uaO.SetOnRemoteSdpChange(self.media.OnCalleeSdpChange)
			self.uaO.SetRAddr(self.destination)
			self.uaO.SetRAddr0(self.destination)
		}
		self.uaO.RecvEvent(event)
	} else {
		self.uaA.RecvEvent(event)
	}
}

func (self *callController) aDead() {
	self.media.Delete()
	self.cmap.Remove(self.id)
}

func (self *callController) oDead() {
	self.media.Delete()
	self.cmap.Remove(self.id)
}

func (self *callController) Shutdown() {
	self.uaA.Disconnect(nil, "")
}

func (self *callController) String() string {
	res := "uaA:" + self.uaA.String() + ", uaO: "
	if self.uaO == nil {
		res += "nil"
	} else {
		res += self.uaO.String()
	}
	return res
}

type callMap struct {
	config     *myconfig
	logger     sippy_log.ErrorLogger
	sip_tm     sippy_types.SipTransactionManager
	proxy      sippy_types.StatefulProxy
	ccmap      map[int64]*callController
	ccmap_lock sync.Mutex
}

func NewCallMap(config *myconfig, logger sippy_log.ErrorLogger) *callMap {
	return &callMap{
		logger: logger,
		config: config,
		ccmap:  make(map[int64]*callController),
	}
}

func (self *callMap) OnNewDialog(req sippy_types.SipRequest, tr sippy_types.ServerTransaction) (sippy_types.UA, sippy_types.RequestReceiver, sippy_types.SipResponse) {
	to_body, err := req.GetTo().GetBody(self.config)
	if err != nil {
		self.logger.Error("CallMap::OnNewDialog: #1: " + err.Error())
		return nil, nil, req.GenResponse(500, "Internal Server Error", nil, nil)
	}
	if to_body.GetTag() != "" {
		// Request within dialog, but no such dialog
		return nil, nil, req.GenResponse(481, "Call Leg/Transaction Does Not Exist", nil, nil)
	}
	if req.GetMethod() == "INVITE" {
		// New dialog
		callerWebRTC := strings.HasSuffix(req.GetSource().Received, ".invalid")
		destination := self.config.nh_addr
		if !callerWebRTC {
			if !strings.HasSuffix(req.GetRURI().Host.String(), ".invalid") {
				return nil, nil, req.GenResponse(403, "WebRTC route required", nil, nil)
			}
			destination = req.GetRURI().GetAddr(self.config)
		}
		cc, err := NewCallController(self, destination, callerWebRTC)
		if err != nil {
			self.logger.Error(err)
			return nil, nil, req.GenResponse(503, "Media Gateway Unavailable", nil, nil)
		}
		self.ccmap_lock.Lock()
		self.ccmap[cc.id] = cc
		self.ccmap_lock.Unlock()
		return cc.uaA, cc.uaA, nil
	}
	if req.GetMethod() == "REGISTER" {
		// Registration
		return nil, self.proxy, nil
	}
	if req.GetMethod() == "NOTIFY" || req.GetMethod() == "PING" {
		// Whynot?
		return nil, nil, req.GenResponse(200, "OK", nil, nil)
	}
	return nil, nil, req.GenResponse(501, "Not Implemented", nil, nil)
}

func (self *callMap) Remove(ccid int64) {
	self.ccmap_lock.Lock()
	defer self.ccmap_lock.Unlock()
	delete(self.ccmap, ccid)
}

func (self *callMap) Shutdown() {
	acalls := []*callController{}
	self.ccmap_lock.Lock()
	for _, cc := range self.ccmap {
		acalls = append(acalls, cc)
	}
	self.ccmap_lock.Unlock()
	for _, cc := range acalls {
		//println(cc.String())
		cc.Shutdown()
	}
}

type myconfig struct {
	sippy_conf.Config

	nh_addr *sippy_net.HostPort
	rtpp    sippy_types.RtpProxyClient
}

func hostPort(value string) (*sippy_net.HostPort, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return nil, err
	}
	return sippy_net.NewHostPort(host, port), nil
}

func run() error {
	var sipAddr, nextHop, registrar, httpsAddr, wssAddr, cert, key, web, rtppAddr string
	flag.StringVar(&sipAddr, "sip", "127.0.0.1:15060", "UDP SIP bind address")
	flag.StringVar(&nextHop, "next-hop", "127.0.0.1:15070", "Outbound SIP route host:port")
	flag.StringVar(&registrar, "registrar", "", "REGISTER proxy host:port (defaults to next-hop)")
	flag.StringVar(&httpsAddr, "https", "0.0.0.0:18443", "HTTPS phone bind address")
	flag.StringVar(&wssAddr, "wss", "0.0.0.0:19876", "WSS SIP bind address")
	flag.StringVar(&cert, "cert", "server.crt", "TLS certificate")
	flag.StringVar(&key, "key", "server.key", "TLS private key")
	flag.StringVar(&web, "web", "web", "Phone static assets")
	flag.StringVar(&rtppAddr, "rtpp", "/tmp/go-webrtc-rtpp.sock", "RTPProxy control socket (unix:/path or udp/tcp address); " +
		"\"rtp.io:[modules=<m1>+<m2>;<arg>;<arg>...]\" runs the engine in-process " +
		"(requires a -tags rtpio build; static module names: dtls_gw, ice_lite; see sippy/rtp_proxy/embedded/README.md)")
	flag.Parse()
	local, err := hostPort(sipAddr)
	if err != nil {
		return err
	}
	next, err := hostPort(nextHop)
	if err != nil {
		return err
	}
	if registrar == "" {
		registrar = nextHop
	}
	reg, err := hostPort(registrar)
	if err != nil {
		return err
	}
	logger := sippy_log.NewErrorLogger()
	sipLogger, err := sippy_log.NewSipLogger("go-webrtc-phone", "/dev/stdout")
	if err != nil {
		return err
	}
	config := &myconfig{Config: sippy_conf.NewConfig(logger, sipLogger), nh_addr: next}
	config.SetIPV6Enabled(false)
	config.SetMyAddress(local.Host)
	config.SetSipAddress(local.Host)
	config.SetMyPort(local.Port)
	config.SetSipPort(local.Port)
	config.SetMyUAName("Sippy Go WebRTC Phone")
	wsAddress, err := hostPort(wssAddr)
	if err != nil {
		return err
	}
	factory := sippy.NewWebSocketTransportFactory(config, wsAddress, cert, key)
	config.SetSipTransportFactory(factory)
	// In-process rtp.io engine (Python-B2BUA-compatible selection): when -rtpp
	// is an "rtp.io:..." spec the statically linked engine starts in-process
	// and the client below connects to the generated control socket exactly
	// like an external rtpproxy (see sippy/rtp_proxy/embedded/README.md).
	clientAddr := rtppAddr
	if internal, spec, perr := embedded.ParseRtpIO(rtppAddr); perr != nil {
		return perr
	} else if internal {
		if !embedded.Available() {
			return fmt.Errorf("rtp.io: (in-process engine) selected via -rtpp, but this binary was built without the rtpio build tag / librtpproxy.a; rebuild with 'go build -tags rtpio' (see sippy/rtp_proxy/embedded/README.md) or use an external rtpproxy control socket")
		}
		ctlSock := "unix:" + filepath.Join(os.TempDir(), fmt.Sprintf("go-webrtc-phone-rtpp-%d.sock", os.Getpid()))
		clientAddr = ctlSock
		// Default RTP port range unless the rtp.io: tokens set -m/-M themselves.
		portMin, portMax := 0, 0
		if !embedded.HasArg(spec.ExtraArgs, "-m") && !embedded.HasArg(spec.ExtraArgs, "-M") {
			portMin, portMax = 12000, 15000
		}
		argv := embedded.EngineArgv(spec, []string{ctlSock}, "", portMin, portMax)
		if err := embedded.Start(argv); err != nil {
			return err
		}
		defer embedded.Stop()
	}
	opts, err := rtp_proxy.NewRtpProxyClientOpts(clientAddr, nil, config, logger)
	if err != nil {
		return err
	}
	rtppClient := rtp_proxy.NewRtpProxyClient(opts)
	config.rtpp = rtppClient
	if err := config.rtpp.Start(); err != nil {
		return err
	}
	defer rtppClient.(interface{ Shutdown() }).Shutdown()
	// The capability handshake is asynchronous. Do not advertise ready before it completes.
	deadline := time.Now().Add(10 * time.Second)
	for !config.rtpp.IsOnline() {
		if time.Now().After(deadline) {
			return fmt.Errorf("RTPProxy not online after 10s")
		}
		time.Sleep(50 * time.Millisecond)
	}
	cmap := NewCallMap(config, logger)
	sipTM, err := sippy.NewSipTransactionManager(config, cmap)
	if err != nil {
		return err
	}
	cmap.sip_tm = sipTM
	cmap.proxy = sippy.NewStatefulProxy(sipTM, reg, config)
	defer sipTM.Shutdown()
	go sipTM.Run()
	server := &http.Server{Addr: httpsAddr, Handler: http.FileServer(http.Dir(web)), ReadHeaderTimeout: 5 * time.Second}
	errors := make(chan error, 2)
	go func() { errors <- server.ListenAndServeTLS(cert, key) }()
	fmt.Printf("Go WebRTC phone: https=%s wss=%s SIP=%s route=%s\n", httpsAddr, wssAddr, sipAddr, nextHop)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	select {
	case <-signals:
	case err := <-errors:
		if err != http.ErrServerClosed {
			return err
		}
	}
	cmap.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
