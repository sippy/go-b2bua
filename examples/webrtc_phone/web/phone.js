// SIP.js is vendored in the Docker build; no third-party requests at runtime.
import { SimpleUser } from './vendor/lib/platform/web/index.js';
const el = id => document.getElementById(id);
const log = message => { el('status').textContent += '\n' + message; };
el('server').value = `wss://${location.hostname}:19876`;
el('certificate').href = `https://${location.hostname}:19876/`;
let phone;
const action = (id, fn) => el(id).onclick = async () => {
  try { await fn(); } catch (e) { log(`ERROR: ${e.message}`); console.error(e); }
};
action('connect', async () => {
  phone = new SimpleUser(el('server').value, {
    aor: el('identity').value,
    media: { constraints: { audio: true, video: false }, remote: { audio: el('remote') } },
    userAgentOptions: {
      authorizationUsername: el('username').value,
      authorizationPassword: el('password').value,
      sessionDescriptionHandlerFactoryOptions: { peerConnectionConfiguration: { iceServers: [] } }
    },
    delegate: {
      onServerConnect: () => log('WSS connected'),
      onServerDisconnect: () => log('WSS disconnected'),
      onRegistered: () => log('Registered'),
      onUnregistered: () => log('Unregistered'),
      onCallReceived: async () => { log('Incoming call'); await phone.answer(); },
      onCallAnswered: () => { log('Call answered'); el('hangup').disabled = false; },
      onCallHangup: () => { log('Call ended'); el('hangup').disabled = true; }
    }
  });
  // Exposed for local demo diagnostics, including getStats on the peer connection.
  window.phone = phone;
  await phone.connect();
  ['register', 'call', 'disconnect'].forEach(id => el(id).disabled = false);
  el('connect').disabled = true;
});
action('register', async () => { await phone.register(); log('REGISTER sent'); });
action('call', async () => { await phone.call(el('destination').value); log('INVITE sent'); });
action('hangup', async () => { await phone.hangup(); });
action('disconnect', async () => {
  if (phone.isConnected()) await phone.disconnect();
  ['register','call','hangup','disconnect'].forEach(id => el(id).disabled = true);
  el('connect').disabled = false;
});
