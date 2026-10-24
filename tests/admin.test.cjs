const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { runInNewContext } = require('node:vm');

const html = readFileSync(new URL('../static/goatadmin.html', `file://${__filename}`), 'utf8');
const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const tick = () => new Promise(resolve => setImmediate(resolve));
const response = (data, status = 200) => ({
  ok: status < 400, status, statusText: String(status),
  text: async () => JSON.stringify(data), json: async () => data
});

// Run the real inline admin script against a small DOM and controllable network.
async function admin(fetchAPI) {
  const elements = new Map();
  const created = [];
  class Element {
    constructor() {
      this.value = '';
      this.textContent = '';
      this.children = [];
      this.listeners = {};
      this.style = {};
      const classes = new Set();
      this.classList = {
        add: name => classes.add(name), remove: name => classes.delete(name),
        contains: name => classes.has(name),
        toggle: (name, on) => on ? classes.add(name) : classes.delete(name)
      };
    }
    addEventListener(name, handler) { this.listeners[name] = handler; }
    appendChild(child) { this.children.push(child); }
    removeAttribute(name) { delete this[name]; }
    focus() {}
    add(option) { this.options.push(option); }
    async fire(name, event = {}) { return this.listeners[name]({ target: this, ...event }); }
  }
  for (const [, id] of html.matchAll(/id="([^"]+)"/g)) elements.set(id, new Element());
  const get = id => elements.get(id);
  get('app-view').classList.add('hidden');
  get('draft-type').options = ['Single', 'Double-Single', 'EP', 'Album', 'Mixtape'].map(value => ({ value }));
  const form = [...elements.entries()].filter(([id]) => id.startsWith('draft-') || ['submit-btn', 'reset-btn'].includes(id)).map(([, el]) => el);
  const requests = [];
  runInNewContext(script, {
    document: {
      getElementById: get,
      querySelectorAll: () => form.concat(created.filter(el => ['btn-edit', 'btn-delete'].includes(el.className))),
      createElement: () => { const el = new Element(); created.push(el); return el; },
      createDocumentFragment: () => new Element()
    },
    window: { scrollTo() {}, confirm: () => true },
    fetch: async (path, init) => {
      requests.push({ path, ...init });
      if (path === '/api/me') return response({ authed: true });
      return fetchAPI(path, init);
    },
    FormData, Date, setTimeout: () => 0, clearTimeout() {},
    Option: function (label, value) { this.value = value; }
  });
  await tick();
  return { get, requests, created };
}

test('saving waits for the cover upload and includes its URL', async () => {
  let finishUpload;
  let payload;
  const app = await admin(async (path, init) => {
    if (path === '/api/upload-cover') return new Promise(resolve => { finishUpload = resolve; });
    if (path === '/api/releases' && init.method === 'POST') {
      payload = JSON.parse(init.body);
      return response({ id: 1, ...payload }, 201);
    }
    return response([]);
  });
  app.get('draft-title').value = 'Upload test';
  app.get('draft-cover-file').files = [new File(['cover'], 'cover.png', { type: 'image/png' })];
  const upload = app.get('draft-cover-file').fire('change');
  assert.equal(app.get('submit-btn').disabled, true);
  await app.get('submit-btn').fire('click');
  assert.equal(payload, undefined);
  finishUpload(response({ url: '/static/uploads/cover.png' }));
  await upload;
  assert.equal(app.get('submit-btn').disabled, false);
  await app.get('submit-btn').fire('click');
  assert.equal(payload.cover, '/static/uploads/cover.png');
  assert.equal(payload.title, 'Upload test');
});

test('a failed save keeps the draft and lets the user retry', async () => {
  let attempts = 0;
  let finishSave;
  const app = await admin(async (path, init) => {
    if (init.method === 'POST') {
      attempts++;
      return new Promise(resolve => { finishSave = resolve; });
    }
    return response([]);
  });
  app.get('draft-title').value = 'Keep my draft';
  const save = app.get('submit-btn').fire('click');
  await app.get('submit-btn').fire('click');
  assert.equal(attempts, 1);
  finishSave(response({ error: 'Disk unavailable' }, 500));
  await save;
  assert.equal(app.get('draft-title').value, 'Keep my draft');
  assert.equal(app.get('form-err').textContent, 'Disk unavailable');
  assert.equal(app.get('submit-btn').disabled, false);
});

test('session expiry preserves the edited draft through login', async () => {
  let expired = true;
  let payload;
  const existing = { id: 3, title: 'Original', release_type: 'album' };
  const app = await admin(async (path, init) => {
    if (path === '/api/login') { expired = false; return response({ ok: true }); }
    if (init.method === 'PUT') {
      if (expired) return response({ error: 'Unauthorized' }, 401);
      payload = JSON.parse(init.body);
      return response({ id: 3, ...payload });
    }
    return response([existing]);
  });
  await app.created.find(el => el.className === 'btn-edit').fire('click');
  assert.equal(app.get('draft-type').value, 'Album');
  app.get('draft-title').value = 'Edited before expiry';
  await app.get('submit-btn').fire('click');
  assert.equal(app.get('app-view').classList.contains('hidden'), true);
  app.get('lock-pass').value = 'password';
  await app.get('lock-btn').fire('click');
  assert.equal(app.get('draft-title').value, 'Edited before expiry');
  assert.equal(app.get('submit-btn').textContent, 'Сохранить');
  await app.get('submit-btn').fire('click');
  assert.equal(payload.title, 'Edited before expiry');
});
