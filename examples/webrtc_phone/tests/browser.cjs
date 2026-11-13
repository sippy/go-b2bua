const { chromium } = require('playwright');
const fs = require('node:fs');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({args:['--use-fake-ui-for-media-stream','--use-fake-device-for-media-stream','--ignore-certificate-errors','--no-sandbox']});
  const logs = [];
  let page;
  try {
    const context = await browser.newContext({ignoreHTTPSErrors:true,permissions:['microphone']});
    await context.addInitScript(() => {
      const Original = window.RTCPeerConnection;
      window.__pcs = [];
      window.RTCPeerConnection = class extends Original { constructor(...args) { super(...args); window.__pcs.push(this); } };
    });
    page = await context.newPage();
    page.on('console',msg => logs.push(msg.text()));
    page.on('pageerror',error => logs.push('PAGE ERROR: '+error.message));
    await page.goto('https://127.0.0.1:18443/');
    await page.getByRole('button',{name:'Connect',exact:true}).click();
    await page.waitForFunction(() => document.getElementById('status').textContent.includes('WSS connected'));
    await page.getByRole('button',{name:'Register',exact:true}).click();
    await page.waitForFunction(() => document.getElementById('status').textContent.includes('Registered'));
    const waitMedia = async () => {
      const until = Date.now() + 20000;
      let mediaReady = false;
      while (Date.now() < until) {
        mediaReady = await page.evaluate(async () => {
          const values = [...(await window.__pcs.at(-1).getStats()).values()];
          return values.some(s => s.type === 'inbound-rtp' && s.packetsReceived > 20) && values.some(s => s.type === 'outbound-rtp' && s.packetsSent > 20);
        });
        if (mediaReady) break;
        await page.waitForTimeout(200);
      }
      assert.ok(mediaReady, 'Bidirectional RTP counters did not grow within 20s');
    };
    const calls = [];
    for (let call = 0; call < 2; call++) {
      await page.getByRole('button',{name:'Call',exact:true}).click();
      await page.waitForFunction(() => window.__pcs.at(-1)?.connectionState === 'connected',null,{timeout:30000});
      await waitMedia();
      const result = await page.evaluate(async () => {
        const pc = window.__pcs.at(-1);
        return { connectionState:pc.connectionState, iceConnectionState:pc.iceConnectionState,
          answer:pc.remoteDescription.sdp, stats:[...await pc.getStats()].map(([,v])=>v).filter(s=>['transport','inbound-rtp','outbound-rtp','candidate-pair'].includes(s.type)) };
      });
      fs.writeFileSync(`/evidence/call-${call}.json`,JSON.stringify(result,null,2));
      assert.match(result.answer,/a=fingerprint:sha-256 /i);
      assert.match(result.answer,/a=ice-lite/);
      assert.match(result.answer,/a=candidate:/);
      assert.ok(result.stats.some(s=>s.type==='transport' && s.dtlsState==='connected'));
      assert.ok(result.stats.some(s=>s.type==='outbound-rtp' && s.packetsSent>20));
      calls.push(result);
      if (call === 0) await page.getByRole('button',{name:'Hang up',exact:true}).click();
      else await fetch('http://127.0.0.1:15071/bye');
      await page.waitForFunction(() => window.__pcs.at(-1)?.connectionState === 'closed',null,{timeout:10000});
    }
    if (process.env.INBOUND_TEST === '1') {
      await fetch('http://127.0.0.1:15071/invite');
      await page.waitForFunction(() => window.__pcs.length >= 3 && window.__pcs.at(-1)?.connectionState === 'connected',null,{timeout:30000});
      await waitMedia();
      calls.push(await page.evaluate(async () => ({direction:'incoming',connectionState:window.__pcs.at(-1).connectionState,stats:[...(await window.__pcs.at(-1).getStats()).values()].filter(s=>['transport','inbound-rtp','outbound-rtp'].includes(s.type))})));
      await page.getByRole('button',{name:'Hang up',exact:true}).click();
      await page.waitForFunction(() => window.__pcs.at(-1)?.connectionState === 'closed',null,{timeout:10000});
    }
    const peer = await (await fetch('http://127.0.0.1:15071/')).json();
    assert.ok(peer.register>0 && peer.invite>=2 && peer.ack>=2 && peer.bye>0 && peer.remoteByeOK>0 && peer.rtpPackets>40,JSON.stringify(peer));
    if (process.env.INBOUND_TEST === '1') assert.ok(peer.inboundAnswered>0);
    for (const offer of peer.offers) {
      assert.match(offer,/m=audio \d+ RTP\/AVP/);
      assert.doesNotMatch(offer,/a=(fingerprint|setup|ice-ufrag|ice-pwd|candidate|rtcp-mux):?/);
      assert.ok(offer.includes(`c=IN IP4 ${process.env.RTP_ADDRESS || '127.0.0.1'}`));
    }
    await page.screenshot({path:'/evidence/phone.png',fullPage:true});
    await page.getByRole('button',{name:'Disconnect',exact:true}).click();
    fs.writeFileSync('/evidence/browser.json',JSON.stringify({verdict:'PASS',calls,peer,logs},null,2));
    console.log(JSON.stringify({verdict:'PASS',calls:calls.length,plainRtpPackets:peer.rtpPackets,register:peer.register,ack:peer.ack,browserBye:peer.bye,remoteByeOK:peer.remoteByeOK}));
  } catch (e) {
    fs.writeFileSync('/evidence/browser-failure.json',JSON.stringify({error:String(e),logs},null,2));
    if (page) await page.screenshot({path:'/evidence/phone-failure.png',fullPage:true}).catch(()=>{});
    throw e;
  } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exit(1)});
