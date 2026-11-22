// ntgamepad - Copyright (c) 2026 Neomantra Corp.
//
// Says what this browser can do with game controllers, from what has been
// tried. It fills the element with id "compat".

// detect names the browser engine and the operating system. It prefers what
// the browser states about itself (userAgentData) and falls back to the
// user-agent string. Feature checks, not this, decide whether the Gamepad
// API is there.
export function detect(nav = navigator) {
  const ua = nav.userAgent || '';
  const brands = (nav.userAgentData?.brands || []).map(b => b.brand);
  let browser = 'other', name = 'This browser';
  if (brands.some(b => /Chromium|Google Chrome|Microsoft Edge|Brave|Opera/.test(b)) || /Chrom(e|ium)\/|Edg\//.test(ua)) {
    browser = 'chromium';
    name = brands.find(b => /Google Chrome|Microsoft Edge|Brave|Opera/.test(b)) || (/Edg\//.test(ua) ? 'Microsoft Edge' : 'Chrome');
  } else if (/Firefox\//.test(ua)) {
    browser = 'firefox';
    name = 'Firefox';
  } else if (/Safari\//.test(ua) && /Apple/.test(nav.vendor || '')) {
    browser = 'safari';
    name = 'Safari';
  }
  const platform = nav.userAgentData?.platform || nav.platform || '';
  let os = 'other', osName = 'this system';
  if (/iPhone|iPad|iPod/.test(ua) || (/Mac/.test(platform) && nav.maxTouchPoints > 1)) {
    os = 'ios'; osName = 'iOS';
  } else if (/Android/.test(ua)) {
    os = 'android'; osName = 'Android';
  } else if (/Mac/i.test(platform)) {
    os = 'macos'; osName = 'macOS';
  } else if (/Win/i.test(platform)) {
    os = 'windows'; osName = 'Windows';
  } else if (/Linux|X11/i.test(platform)) {
    os = 'linux'; osName = 'Linux';
  }
  return { browser, name, os, osName };
}

// report is the list of findings for a browser: each has a level ("yes",
// "no" or "unknown"), a subject and what is known about it.
export function report(d, hasGamepads, secure) {
  if (!hasGamepads) {
    return [{ level: 'no', subject: 'Controllers', text: secure
      ? 'This browser has no Gamepad API, so a page cannot read a controller.'
      : 'The Gamepad API needs a secure page (https or localhost).' }];
  }
  const rows = [{
    level: 'yes', subject: 'Standard controllers',
    text: 'Any controller the browser gives its standard mapping is read directly. Press a button on it first: a browser hides controllers from a page until then.',
  }];
  const pad = 'Switch 2 Pro Controller';
  const needsStart = 'The pad sends nothing until a native program has started it since it was plugged in: run ntgamepad-probe, or anything else built on ntgamepad, once.';
  if (d.os !== 'macos') {
    rows.push({ level: 'unknown', subject: pad, text: `Not tried on ${d.osName}. ntgamepad's driver for this pad is macOS only so far.` });
  } else if (d.browser === 'chromium') {
    rows.push({ level: 'yes', subject: pad, text: `${d.name} lists it over USB as a generic gamepad and ntgamepad maps it. ${needsStart}` });
  } else if (d.browser === 'firefox') {
    rows.push({ level: 'yes', subject: pad, text: `Firefox lists it over USB as a generic gamepad and ntgamepad maps it. ${needsStart}` });
  } else if (d.browser === 'safari') {
    rows.push({ level: 'no', subject: pad, text: 'Safari takes its controllers from macOS, which does not recognise this pad, so a page never sees it. Use a Chromium browser or Firefox.' });
  } else {
    rows.push({ level: 'unknown', subject: pad, text: 'Not tried in this browser.' });
  }
  return rows;
}

const labels = { yes: 'Works', no: 'Does not work', unknown: 'Not tried' };

export function render(el, nav = navigator) {
  if (!el) return;
  const d = detect(nav);
  const rows = report(d, typeof nav.getGamepads === 'function', window.isSecureContext);
  el.textContent = '';
  const h = document.createElement('h2');
  h.textContent = `${d.name} on ${d.osName}`;
  el.append(h);
  const list = document.createElement('ul');
  for (const r of rows) {
    const li = document.createElement('li');
    li.dataset.level = r.level;
    const badge = document.createElement('span');
    badge.className = 'compat-level';
    badge.textContent = labels[r.level];
    const subject = document.createElement('strong');
    subject.textContent = r.subject;
    li.append(badge, ' ', subject, ': ', r.text);
    list.append(li);
  }
  el.append(list);
}

if (typeof document !== 'undefined') {
  render(document.querySelector('#compat'));
}
