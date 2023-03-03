package sippy

import (
    "sync"
    "testing"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/log"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/types"
)

type test_sip_logger struct {
}

type test_call_map struct {
    config      sippy_conf.Config
    sip_tm      sippy_types.SipTransactionManager
    ua          sippy_types.UA
    lock        sync.Mutex
    msg_body    sippy_types.MsgBody
}

func NewTestSipLogger() sippy_log.SipLogger {
    return &test_sip_logger{}
}

func (*test_sip_logger) Write(*sippy_time.MonoTime, string, string) {
}

func NewTestCallMap(config sippy_conf.Config) *test_call_map {
    return &test_call_map{
        config      : config,
    }
}

func (self *test_call_map) OnNewDialog(req sippy_types.SipRequest, tr sippy_types.ServerTransaction) (sippy_types.UA, sippy_types.RequestReceiver, sippy_types.SipResponse) {
    self.ua = NewUA(self.sip_tm, self.config, sippy_net.NewHostPort("1.1.1.1", "5060"), self, &self.lock, nil)
    self.msg_body = req.GetBody()
    return self.ua, self.ua, nil
}

func (self *test_call_map) RecvEvent(sippy_types.CCEvent, sippy_types.UA) {
}

func (self *test_call_map) disconnect() {
    self.ua.Disconnect(nil, "")
}

func (self *test_call_map) answer() {
    rtime, _ := sippy_time.NewMonoTime()
    self.ua.RecvEvent(NewCCEventConnect(200, "OK", self.msg_body, rtime, "caller"))
}

func Test_LooseRouting(t *testing.T) {
    var err error

    error_logger := sippy_log.NewErrorLogger()
    sip_logger := NewTestSipLogger()
    config := sippy_conf.NewConfig(error_logger, sip_logger)
    config.SetSipAddress(config.GetMyAddress())
    config.SetSipPort(config.GetMyPort())
    cmap := NewTestCallMap(config)
    tfactory := NewTestSipTransportFactory()
    config.SetSipTransportFactory(tfactory)
    cmap.sip_tm, err = NewSipTransactionManager(config, cmap)
    if err != nil {
        t.Fatal("Cannot create SIP transaction manager: " + err.Error())
    }
    go cmap.sip_tm.Run()
    defer cmap.sip_tm.Shutdown()
    tfactory.feed([]string{
        "INVITE sip:905399232076@10.20.30.40 SIP/2.0",
        "Record-Route: <sip:2.2.2.2;r2=on;lr=on;ftag=as57b03f0f>",
        "Record-Route: <sip:10.112.247.248;r2=on;lr=on;ftag=as57b03f0f>",
        "Via: SIP/2.0/UDP 2.2.2.2;branch=z9hG4bKb9de.5d673b5c814b2bfbfb6d44b3e414d31c.0",
        "Via: SIP/2.0/UDP 10.164.244.209:5060;received=10.164.244.209;branch=z9hG4bK0b5aac35;rport=5060",
        "Max-Forwards: 69",
        "From: \"John Smith\" <sip:testcli@sip.test.com>;tag=as57b03f0f",
        "To: <sip:905399232076@sip-carriers.local>",
        "Contact: <sip:908502729000@10.164.244.209:5060>",
        "Call-ID: 699e676c1826a8323e705cbd6c61c3e6@sip.test.com",
        "CSeq: 102 INVITE",
        "User-Agent: Unit Test",
        "Date: Thu, 09 Nov 2017 15:49:51 GMT",
        "Allow: INVITE, ACK, CANCEL, OPTIONS, BYE, REFER, SUBSCRIBE, NOTIFY, INFO, PUBLISH, MESSAGE",
        "Content-Type: application/sdp",
        "Content-Length: 126",
        "",
        "v=0",
        "o=user1 53655765 2353687637 IN IP4 1.1.1.1",
        "s=-",
        "c=IN IP4 1.1.1.1",
        "t=0 0",
        "m=audio 11111 RTP/AVP 0",
        "a=rtpmap:0 PCMU/8000",
        "",
    })
    tfactory.get() // 100 Trying
    cmap.answer()
    tfactory.get() // 200 OK
    cmap.disconnect()
    res := tfactory.get() // BYE
    rtime, _ := sippy_time.NewMonoTime()
    bye, err := ParseSipRequest(res, rtime, config)
    if err != nil {
        t.Fatal("Cannot parse BYE: " + err.Error())
    }
    if len(bye.routes) != 2 {
        t.Fatal("The number of routes in BYE is not 2")
    }
    assertStringEqual(bye.GetRURI().Host.String(), "10.164.244.209", t)
    assertStringEqual(bye.GetRURI().Username, "908502729000", t)
}

func Test_StrictRouting(t *testing.T) {
    var err error

    error_logger := sippy_log.NewErrorLogger()
    sip_logger := NewTestSipLogger()
    config := sippy_conf.NewConfig(error_logger, sip_logger)
    config.SetSipAddress(config.GetMyAddress())
    config.SetSipPort(config.GetMyPort())
    cmap := NewTestCallMap(config)
    tfactory := NewTestSipTransportFactory()
    config.SetSipTransportFactory(tfactory)
    cmap.sip_tm, err = NewSipTransactionManager(config, cmap)
    defer cmap.sip_tm.Shutdown()
    if err != nil {
        t.Fatal("Cannot create SIP transaction manager: " + err.Error())
    }
    go cmap.sip_tm.Run()
    tfactory.feed([]string{
        "INVITE sip:905399232076@10.20.30.40 SIP/2.0",
        "Record-Route: <sip:2.2.2.2;r2=on;ftag=as57b03f0f>",
        "Record-Route: <sip:10.112.247.248;r2=on;lr=on;ftag=as57b03f0f>",
        "Via: SIP/2.0/UDP 2.2.2.2;branch=z9hG4bKb9de.5d673b5c814b2bfbfb6d44b3e414d31c.0",
        "Via: SIP/2.0/UDP 10.164.244.209:5060;received=10.164.244.209;branch=z9hG4bK0b5aac35;rport=5060",
        "Max-Forwards: 69",
        "From: \"John Smith\" <sip:testcli@sip.test.com>;tag=as57b03f0f",
        "To: <sip:905399232076@sip-carriers.local>",
        "Contact: <sip:908502729000@10.164.244.209:5060>",
        "Call-ID: 699e676c1826a8323e705cbd6c61c3e6@sip.test.com",
        "CSeq: 102 INVITE",
        "User-Agent: Unit Test",
        "Date: Thu, 09 Nov 2017 15:49:51 GMT",
        "Allow: INVITE, ACK, CANCEL, OPTIONS, BYE, REFER, SUBSCRIBE, NOTIFY, INFO, PUBLISH, MESSAGE",
        "Content-Type: application/sdp",
        "Content-Length: 126",
        "",
        "v=0",
        "o=user1 53655765 2353687637 IN IP4 1.1.1.1",
        "s=-",
        "c=IN IP4 1.1.1.1",
        "t=0 0",
        "m=audio 11111 RTP/AVP 0",
        "a=rtpmap:0 PCMU/8000",
        "",
    })
    tfactory.get() // 100 Trying
    cmap.answer()
    tfactory.get() // 200 OK
    cmap.disconnect()
    res := tfactory.get() // BYE
    rtime, _ := sippy_time.NewMonoTime()
    bye, err := ParseSipRequest(res, rtime, config)
    if err != nil {
        t.Fatal("Cannot parse BYE: " + err.Error())
    }
    if len(bye.routes) != 2 {
        t.Fatal("The number of routes in BYE is not 2")
    }
    addr, err := bye.routes[1].GetBody(config)
    if err != nil {
        t.Fatal("Cannot parse Route: " + err.Error())
    }
    // check the last route points to the Contact: from INVITE
    assertStringEqual(addr.GetUrl().Host.String(), "10.164.244.209", t)
    assertStringEqual(addr.GetUrl().Username, "908502729000", t)
    assertStringEqual(bye.GetRURI().Host.String(), "2.2.2.2", t)
    assertStringEqual(bye.GetRURI().Username, "", t)
}

func assertStringEqual(check, expect string, t *testing.T) {
    if check == expect {
        return
    }
    t.Fatalf("Got %s while expecting %s", check, expect)
}
