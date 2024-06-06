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
    "errors"
    "flag"
    "strconv"
    "strings"
    "time"

    "github.com/gookit/ini/v2"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/log"
    "github.com/sippy/go-b2bua/sippy/net"
)

const (
    INI_SECTION = "general"
)

type myConfigParser struct {
    sippy_conf.Config

    Accept_ips_map      map[string]bool
    Hrtb_retr_ival_dur  time.Duration
    Hrtb_ival_dur       time.Duration
    Keepalive_ans_dur   time.Duration
    Keepalive_orig_dur  time.Duration
    Rtp_proxy_clients_arr []string
    Pass_headers_arr    []string
    Allowed_pts_map     map[string]bool

    Accept_ips          string
    Acct_enable         bool
    Alive_acct_int      int
    Allowed_pts         string
    Auth_enable         bool
    B2bua_socket        string
    Foreground          bool
    Hide_call_id        bool
    Hrtb_retr_ival      int
    Hrtb_ival           int
    Keepalive_ans       int
    Keepalive_orig      int
    Logfile             string
    Max_credit_time     int
    Max_radius_clients  int
    Nat_traversal       bool
    Precise_acct        bool
    Digest_auth         bool
    Digest_auth_only    bool
    Pass_headers        string
    Pidfile             string
    Radiusclient        string
    Radiusclient_conf   string
    Rtp_proxy_clients   string
    Rtpp_hrtb_ival      int
    Rtpp_hrtb_retr_ival int
    Static_route        string
    Sip_address         string
    Static_tr_in        string
    Static_tr_out       string
    Sip_port            int
    Sip_proxy           string
    Start_acct_enable   bool

    bool_opts           []_bool_opt
    int_opts            []_int_opt
    str_opts            []_str_opt
}

type _bool_opt struct {
    opt_name    string
    descr       string
    ptr         *bool
    def_val     bool
}

type _int_opt struct {
    opt_name    string
    descr       string
    ptr         *int
    def_val     int
}

type _str_opt struct {
    opt_name    string
    descr       string
    ptr         *string
    def_val     string
}

func NewMyConfigParser() *myConfigParser {
    self := &myConfigParser{
        Rtp_proxy_clients_arr : make([]string, 0),
        Accept_ips_map      : make(map[string]bool),
        Auth_enable         : true,
        Acct_enable         : false,
        Start_acct_enable   : false,
        Pass_headers_arr    : make([]string, 0),
        Allowed_pts_map     : make(map[string]bool),
    }
    self.bool_opts = []_bool_opt{
        { "acct_enable", "enable or disable Radius accounting", &self.Acct_enable, true },
        { "precise_acct", "do Radius accounting with millisecond precision", &self.Precise_acct, false },
        { "auth_enable", "enable or disable Radius authentication", &self.Auth_enable, true },
        { "digest_auth", "enable or disable SIP Digest authentication of incoming INVITE requests", &self.Digest_auth, true },
        { "foreground", "run in foreground", &self.Foreground, false },
        { "hide_call_id", "do not pass Call-ID header value from ingress call leg to egress call leg", &self.Hide_call_id, false },
        { "start_acct_enable", "enable start Radius accounting", &self.Start_acct_enable, false },
        { "digest_auth_only", "only use SIP Digest method to authenticate " +
                             "incoming INVITE requests. If the option is not " +
                             "specified or set to \"off\" then B2BUA will try to " +
                             "do remote IP authentication first and if that fails " +
                             "then send a challenge and re-authenticate when " +
                             "challenge response comes in", &self.Digest_auth_only, false },
        { "nat_traversal", "enable NAT traversal for signalling", &self.Nat_traversal, false },
    }
    self.int_opts = []_int_opt{
        { "alive_acct_int", "interval for sending alive Radius accounting in " +
                             "second (0 to disable alive accounting)", &self.Alive_acct_int, -1 },
        { "keepalive_ans", "send periodic \"keep-alive\" re-INVITE requests on " +
                             "answering (ingress) call leg and disconnect a call " +
                             "if the re-INVITE fails (period in seconds, 0 to " +
                             "disable)", &self.Keepalive_ans, 0 },
        { "keepalive_orig", "send periodic \"keep-alive\" re-INVITE requests on " +
                             "originating (egress) call leg and disconnect a call " +
                             "if the re-INVITE fails (period in seconds, 0 to " +
                             "disable)", &self.Keepalive_orig, 0 },
        { "max_credit_time", "upper limit of session time for all calls in seconds", &self.Max_credit_time, -1 },
        { "max_radiusclients", "maximum number of Radius Client helper " +
                             "processes to start", &self.Max_radius_clients, 20 },
        { "sip_port", "local UDP port to listen for incoming SIP requests", &self.Sip_port, 5060 },
        { "rtpp_hrtb_ival", "rtpproxy hearbeat interval (seconds)", &self.Rtpp_hrtb_ival, 10 },
        { "rtpp_hrtb_retr_ival", "rtpproxy hearbeat retry interval (seconds)", &self.Rtpp_hrtb_retr_ival, 60 },
    }
    self.str_opts = []_str_opt{
        { "b2bua_socket", "path to the B2BUA command socket or address to listen " +
                             "for commands in the format \"udp:host[:port]\"", &self.B2bua_socket, "/var/run/b2bua.sock" },
        { "logfile", "path to the B2BUA log file", &self.Logfile, "/var/log/b2bua.log" },
        { "pidfile", "path to the B2BUA PID file", &self.Pidfile, "/var/run/b2bua.pid" },
        { "radiusclient", "path to the radiusclient executable", &self.Radiusclient, "/usr/local/sbin/radiusclient" },
        { "radiusclient_conf", "path to the radiusclient.conf file", &self.Radiusclient_conf, "" },
        { "sip_address", "local SIP address to listen for incoming SIP requests " +
                             "(\"*\", \"0.0.0.0\" or \"::\" to listen on all IPv4 " +
                             "or IPv6 interfaces)", &self.Sip_address, "" },
        { "static_route", "static route for all SIP calls", &self.Static_route, "" },
        { "static_tr_in", "translation rule (regexp) to apply to all incoming " +
                             "(ingress) destination numbers", &self.Static_tr_in, "" },
        { "static_tr_out", "translation rule (regexp) to apply to all outgoing " +
                             "(egress) destination numbers", &self.Static_tr_out, "" },
        { "allowed_pts", "list of allowed media (RTP) IANA-assigned payload " +
                             "types that the B2BUA will pass from input to " +
                             "output, payload types not in this list will be " +
                             "filtered out (comma separated list)", &self.Allowed_pts, "" },
        { "accept_ips", "IP addresses that we will only be accepting incoming " +
                             "calls from (comma-separated list). If the parameter " +
                             "is not specified, we will accept from any IP and " +
                             "then either try to authenticate if authentication " +
                             "is enabled, or just let them to pass through", &self.Accept_ips, "" },
        { "sip_proxy", "address of the helper proxy to handle \"REGISTER\" " +
                             "and \"SUBSCRIBE\" messages. Address in the format " +
                             "\"host[:port]\"", &self.Sip_proxy, "" },
        { "pass_headers", "list of SIP header field names that the B2BUA will " +
                             "pass from ingress call leg to egress call leg " +
                             "unmodified (comma-separated list)", &self.Pass_headers, "" },
        { "rtp_proxy_clients", "comma-separated list of paths or addresses of the " +
                             "RTPproxy control socket. Address in the format " +
                             "\"udp:host[:port]\" (comma-separated list)", &self.Rtp_proxy_clients, "" },
    }
    return self
}

func (self *myConfigParser) Parse() error {
    auth_disable := false
    acct_level := int(-1)
    var disable_digest_auth bool
    var ka_level int

    self.setupOpts()
    flag.BoolVar(&self.Foreground, "f", false, "see -foreground")
    flag.StringVar(&self.Sip_address, "l", "", "see -sip_address")
    flag.StringVar(&self.Pidfile, "P", "/var/run/b2bua.pid", "see -pidfile")
    flag.StringVar(&self.Logfile, "L", "/var/log/sip.log", "see -logfile")
    flag.StringVar(&self.Static_route, "s", "", "see -static_route")
    flag.StringVar(&self.Accept_ips, "a", "", "see -accept_ips")
    flag.BoolVar(&disable_digest_auth, "D", false, "disable digest authentication")
    flag.StringVar(&self.Static_tr_in, "t", "", "see -static_tr_in")
    flag.StringVar(&self.Static_tr_out, "T", "", "see -static_tr_out")
    flag.IntVar(&ka_level, "k", 0, "keepalive level")
    flag.IntVar(&self.Max_credit_time, "m", -1, "see -max_credit_time")
    flag.BoolVar(&auth_disable, "u", false, "disable RADIUS authentication")
    flag.IntVar(&acct_level, "A", -1, "RADIUS accounting level")
    flag.StringVar(&self.Allowed_pts, "F", "", "see -allowed_pts")
    flag.StringVar(&self.Radiusclient_conf, "R", "", "see -radiusclient_conf")
    flag.StringVar(&self.B2bua_socket, "c", "/var/run/b2bua.sock", "see -b2bua_socket")
    flag.IntVar(&self.Max_radius_clients, "M", 1, "see -max_radiusclients")
    flag.BoolVar(&self.Hide_call_id, "H", false, "see -hide_call_id")
    flag.IntVar(&self.Sip_port, "p", 5060, "see -sip_port")

    var writeconf string
    flag.StringVar(&writeconf, "W", "", "Config file name to write the config to")

    var readconf string
    flag.StringVar(&readconf, "C", "", "Config file name to read the config from")
    flag.StringVar(&readconf, "config", "", "Config file name to read the config from")

    // Everything's prepared. Now parse it.
    flag.Parse()

    for _, pt := range strings.Split(self.Allowed_pts, ",") {
        pt = strings.TrimSpace(pt)
        self.Allowed_pts_map[pt] = true
    }
    if self.Sip_address != "" {
        self.SetSipAddress(sippy_net.NewMyAddress(self.Sip_address))
    }
    if disable_digest_auth {
        self.Digest_auth = false
    }
    self.try_read(strings.TrimSpace(readconf))

    if self.Sip_port <= 0 || self.Sip_port > 65535 {
        return errors.New("sip_port should be in the range 1-65535")
    }

    arr := strings.Split(self.Rtp_proxy_clients, ",")
    for _, s := range arr {
        s = strings.TrimSpace(s)
        if s != "" {
            self.Rtp_proxy_clients_arr = append(self.Rtp_proxy_clients_arr, s)
        }
    }
    arr = strings.Split(self.Accept_ips, ",")
    for _, s := range arr {
        s = strings.TrimSpace(s)
        if s != "" {
            self.Accept_ips_map[s] = true
        }
    }
    arr = strings.Split(self.Pass_headers, ",")
    for _, s := range arr {
        s = strings.TrimSpace(s)
        if s != "" {
            self.Pass_headers_arr = append(self.Pass_headers_arr, s)
        }
    }
    switch ka_level {
    case 0:
        // do nothing
    case 1:
        self.Keepalive_ans_dur = 32 * time.Second
    case 2:
        self.Keepalive_orig_dur = 32 * time.Second
    case 3:
        self.Keepalive_ans_dur = 32 * time.Second
        self.Keepalive_orig_dur = 32 * time.Second
    default:
        return errors.New("-k argument not in the range 0-3")
    }
    if self.Keepalive_ans > 0 {
        self.Keepalive_ans_dur = time.Duration(self.Keepalive_ans) * time.Second
    } else if self.Keepalive_ans < 0 {
        return errors.New("keepalive_ans should be non-negative")
    }
    if self.Keepalive_orig > 0 {
        self.Keepalive_orig_dur = time.Duration(self.Keepalive_orig) * time.Second
    } else if self.Keepalive_orig < 0 {
        return errors.New("keepalive_orig should be non-negative")
    }
    if self.Max_credit_time < 0 && self.Max_credit_time != -1 {
        return errors.New("max_credit_time should be more than zero")
    }
    error_logger := sippy_log.NewErrorLogger()
    sip_logger, err := sippy_log.NewSipLogger("b2bua", self.Logfile)
    if err != nil {
        return err
    }
    self.Hrtb_ival_dur = time.Duration(self.Hrtb_ival) * time.Second
    self.Hrtb_retr_ival_dur = time.Duration(self.Hrtb_retr_ival) * time.Second
    self.Config = sippy_conf.NewConfig(error_logger, sip_logger)
    self.SetMyPort(sippy_net.NewMyPort(strconv.Itoa(self.Sip_port)))
    if auth_disable {
        self.Auth_enable = false
    }
    switch acct_level {
    case -1:
        // option is not set
    case 0:
        self.Acct_enable = false
        self.Start_acct_enable = false
    case 1:
        self.Acct_enable = true
        self.Start_acct_enable = false
    case 2:
        self.Acct_enable = true
        self.Start_acct_enable = true
    default:
        return errors.New("-A argument not in the range 0-2")
    }
    self.try_write(strings.TrimSpace(writeconf))
    return nil
}

func (self *myConfigParser) checkIP(ip string) bool {
    if len(self.Accept_ips_map) == 0 {
        return true
    }
    _, ok := self.Accept_ips_map[ip]
    return ok
}

func (self *myConfigParser) try_write(fname string) error {
    if fname == "" {
        return nil
    }
    writer := ini.New()
    for _, opt := range self.bool_opts {
        writer.Set(opt.opt_name, *opt.ptr, INI_SECTION)
    }
    for _, opt := range self.int_opts {
        writer.Set(opt.opt_name, *opt.ptr, INI_SECTION)
    }
    for _, opt := range self.str_opts {
        writer.Set(opt.opt_name, *opt.ptr, INI_SECTION)
    }
    _, err := writer.WriteToFile(fname)
    return err
}

func (self *myConfigParser) try_read(fname string) error {
    if fname == "" {
        return nil
    }
    reader := ini.New()
    err := reader.LoadFiles(fname)
    if err != nil {
        return err
    }
    for _, opt := range self.bool_opts {
        *opt.ptr = reader.Bool(INI_SECTION + "." + opt.opt_name, opt.def_val)
    }
    for _, opt := range self.int_opts {
        val, err := strconv.Atoi(reader.Get(INI_SECTION + "." + opt.opt_name, strconv.Itoa(opt.def_val)))
        if err == nil {
            *opt.ptr = val
        }
    }
    for _, opt := range self.str_opts {
        *opt.ptr = reader.Get(INI_SECTION + "." + opt.opt_name, opt.def_val)
    }
    return err
}

func (self *myConfigParser) setupOpts() {
    for _, opt := range self.bool_opts {
        flag.BoolVar(opt.ptr, opt.opt_name, opt.def_val, opt.descr)
    }
    for _, opt := range self.int_opts {
        flag.IntVar(opt.ptr, opt.opt_name, opt.def_val, opt.descr)
    }
    for _, opt := range self.str_opts {
        flag.StringVar(opt.ptr, opt.opt_name, opt.def_val, opt.descr)
    }
    flag.Func("r", "RTPproxy control socket. See -rtp_proxy_clients for more information", self._rtp_proxy_client_cb)
    flag.Func("rtp_proxy_client", "RTPproxy control socket. See -rtp_proxy_clients for more information", self._rtp_proxy_client_cb)
    flag.Func("h", "SIP header field name to pass from ingress call leg to egress call leg unmodified", self._pass_header_cb)
    flag.Func("pass_header", "SIP header field name to pass from ingress call leg to egress call leg unmodified", self._pass_header_cb)
}

func (self *myConfigParser) _rtp_proxy_client_cb(val string) error {
    val = strings.TrimSpace(val)
    if val == "" {
        return nil
    }
    self.Rtp_proxy_clients_arr = append(self.Rtp_proxy_clients_arr, val)
    return nil
}

func (self *myConfigParser) _pass_header_cb(val string) error {
    val = strings.TrimSpace(val)
    if val == "" {
        return nil
    }
    self.Pass_headers_arr = append(self.Pass_headers_arr, val)
    return nil
}
