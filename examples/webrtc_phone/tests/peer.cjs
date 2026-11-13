// Disposable SIP/UDP registrar + RTP echo peer. Runs only inside Docker.
const dgram = require('node:dgram');
const http = require('node:http');
const sip = dgram.createSocket('udp4');
const rtp = dgram.createSocket('udp4');
const state = { register: 0, invite: 0, ack: 0, bye: 0, remoteByeOK: 0, inboundAnswered: 0, rtpPackets: 0, rtpBytes: 0, offers: [] };
let dialog, registration, incoming;
const plainSDP = () => ['v=0', 'o=echo 1 1 IN IP4 127.0.0.1', 's=RTP echo', 'c=IN IP4 127.0.0.1', 't=0 0',
  'm=audio 36000 RTP/AVP 0', 'a=rtpmap:0 PCMU/8000', 'a=sendrecv', ''].join('\r\n');
const header = (msg, name) => (msg.match(new RegExp(`^${name}:\\s*(.+)$`, 'im')) || [,''])[1].trim();
const respond = (msg, remote, code = 200, body = '') => {
  const vias = msg.split('\r\n').filter(x => /^via:/i.test(x));
  let to = header(msg, 'To');
  if (!/;tag=/i.test(to)) to += ';tag=echo';
  const response = [ `SIP/2.0 ${code} OK`, ...vias, `From: ${header(msg,'From')}`, `To: ${to}`,
    `Call-ID: ${header(msg,'Call-ID')}`, `CSeq: ${header(msg,'CSeq')}`,
    `Contact: ${msg.startsWith('REGISTER ') ? header(msg,'Contact') : '<sip:echo@127.0.0.1:15070>'}`, ...(body ? ['Content-Type: application/sdp'] : []),
    `Content-Length: ${Buffer.byteLength(body)}`, '', body ].join('\r\n');
  sip.send(response, remote.port, remote.address);
};
sip.on('message', (data, remote) => {
  const msg = data.toString();
  const method = msg.split(' ')[0];
  if (method === 'REGISTER') { state.register++; registration = {msg,remote}; respond(msg, remote); }
  if (method === 'INVITE') {
    state.invite++;
    const offer = msg.split('\r\n\r\n')[1] || '';
    state.offers.push(offer);
    // Select G.711 PCMU. The browser and RTPProxy support it without transcoding.
    const answer = plainSDP();
    dialog = { msg, remote };
    respond(msg, remote, 200, answer);
  }
  if (method === 'ACK') state.ack++;
  if (method === 'BYE') { state.bye++; respond(msg, remote); dialog = undefined; }
  if (msg.startsWith('SIP/2.0 200') && header(msg,'CSeq').endsWith('BYE')) { state.remoteByeOK++; dialog = undefined; }
  if (msg.startsWith('SIP/2.0 200') && incoming && header(msg,'Call-ID') === incoming.callId && header(msg,'CSeq').endsWith('INVITE')) {
    state.inboundAnswered++;
    const target = header(msg,'Contact').match(/<([^>]+)>/)[1];
    const ack = [`ACK ${target} SIP/2.0`, 'Via: SIP/2.0/UDP 127.0.0.1:15070;branch=z9hG4bKack'+Date.now(),
      `From: ${incoming.from}`, `To: ${header(msg,'To')}`, `Call-ID: ${incoming.callId}`,
      'CSeq: 1 ACK', 'Max-Forwards: 70', 'Content-Length: 0', '', ''].join('\r\n');
    sip.send(ack,remote.port,remote.address);
  }
});
rtp.on('message', (data, remote) => {
  state.rtpPackets++;
  state.rtpBytes += data.length;
  rtp.send(data, remote.port, remote.address);
});
http.createServer((req,res) => {
  if (req.url === '/invite' && registration) {
    const {msg,remote} = registration;
    const target = header(msg,'Contact').match(/<([^>]+)>/)[1];
    const body = plainSDP();
    incoming = {callId:'inbound-'+Date.now(),from:'<sip:echo@localhost>;tag=peer'+Date.now()};
    const invite = [`INVITE ${target} SIP/2.0`, 'Via: SIP/2.0/UDP 127.0.0.1:15070;branch=z9hG4bKinbound'+Date.now(),
      `From: ${incoming.from}`, `To: ${header(msg,'To')}`, `Call-ID: ${incoming.callId}`,
      'CSeq: 1 INVITE', 'Max-Forwards: 70', 'Contact: <sip:echo@127.0.0.1:15070>',
      'Content-Type: application/sdp',`Content-Length: ${Buffer.byteLength(body)}`,'',body].join('\r\n');
    sip.send(invite,remote.port,remote.address);
  }
  if (req.url === '/bye' && dialog) {
    const {msg,remote} = dialog;
    const target = header(msg,'Contact').match(/<([^>]+)>/)[1];
    const bye = [`BYE ${target} SIP/2.0`, 'Via: SIP/2.0/UDP 127.0.0.1:15070;branch=z9hG4bKpeerbye'+Date.now(),
      `From: ${header(msg,'To')};tag=echo`, `To: ${header(msg,'From')}`, `Call-ID: ${header(msg,'Call-ID')}`,
      'CSeq: 100 BYE', 'Max-Forwards: 70', 'Content-Length: 0', '', ''].join('\r\n');
    sip.send(bye,remote.port,remote.address);
  }
  res.setHeader('Content-Type','application/json'); res.end(JSON.stringify(state));
}).listen(15071,'127.0.0.1');
sip.bind(15070,'127.0.0.1');
rtp.bind(36000,'127.0.0.1');
console.log('Disposable SIP/RTP echo peer ready on UDP15070/RTP36000, stats HTTP15071');
