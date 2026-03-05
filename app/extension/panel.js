/**
 * DUMP panel: load WASM, decode Protobuf, render searchable JSON tree.
 */
(function () {
  const statusEl = document.getElementById('status');
  const contentEl = document.getElementById('content');
  const searchEl = document.getElementById('search');
  const decodeLastBtn = document.getElementById('decode-last');
  const loadDemoBtn = document.getElementById('load-demo');

  let decodeProtobuf = null;

  function setStatus(msg) {
    statusEl.textContent = msg;
  }

  function renderTree(obj, searchTerm) {
    const term = (searchTerm || '').toLowerCase();
    function matches(val) {
      if (term === '') return true;
      const s = typeof val === 'object' ? JSON.stringify(val) : String(val);
      return s.toLowerCase().includes(term);
    }
    function row(key, val, depth) {
      const k = String(key);
      if (val === null || val === undefined) {
        const visible = term === '' || k.toLowerCase().includes(term) || matches('null');
        return visible ? `<li><span class="key">${escapeHtml(k)}</span>: <span class="null">null</span></li>` : '';
      }
      if (typeof val === 'object' && !Array.isArray(val) && val !== null) {
        const childRows = Object.entries(val).map(([cK, cV]) => row(cK, cV, depth + 1)).filter(Boolean).join('');
        const open = term !== '' ? ' open' : '';
        const label = Array.isArray(val) ? '[]' : '{}';
        return `<li class="toggle${open}" data-depth="${depth}"><span class="key">${escapeHtml(k)}</span>: ${label}<ul>${childRows || '<li class="null">(empty)</li>'}</ul></li>`;
      }
      if (Array.isArray(val)) {
        const childRows = val.map((v, i) => row(i, v, depth + 1)).filter(Boolean).join('');
        const open = term !== '' ? ' open' : '';
        return `<li class="toggle${open}" data-depth="${depth}"><span class="key">${escapeHtml(k)}</span>: [<ul>${childRows || '<li class="null">(empty)</li>'}</ul>]</li>`;
      }
      const visible = matches(val) || k.toLowerCase().includes(term);
      if (!visible) return '';
      let cls = 'number';
      if (typeof val === 'string') cls = 'string';
      else if (typeof val === 'boolean') cls = 'bool';
      const display = typeof val === 'string' ? '"' + escapeHtml(val) + '"' : String(val);
      return `<li><span class="key">${escapeHtml(k)}</span>: <span class="${cls}">${display}</span></li>`;
    }
    const html = Object.entries(obj).map(([k, v]) => row(k, v, 0)).filter(Boolean).join('');
    contentEl.innerHTML = html ? `<ul class="tree">${html}</ul>` : '<div class="error">No keys (or nothing matches search).</div>';
    contentEl.querySelectorAll('.tree .toggle').forEach(el => {
      el.addEventListener('click', () => el.classList.toggle('open'));
    });
  }

  function escapeHtml(s) {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  function showResult(result) {
    if (result && typeof result === 'object' && result.error) {
      contentEl.innerHTML = '<div class="error">' + escapeHtml(result.error) + '</div>';
      setStatus('Decode error.');
      window.__dumpLastDecoded = null;
      return;
    }
    if (result && typeof result === 'object') {
      window.__dumpLastDecoded = result;
      renderTree(result, searchEl.value);
      setStatus('Decoded Protobuf (heuristic). Use search to filter.');
      return;
    }
    setStatus('No result.');
    window.__dumpLastDecoded = null;
  }

  function decodeBytes(uint8Array) {
    if (!decodeProtobuf) {
      contentEl.innerHTML = '<div class="error">WASM not loaded yet. Reload the extension or open DUMP Proto panel again.</div>';
      return;
    }
    try {
      const result = decodeProtobuf(uint8Array);
      showResult(result);
    } catch (e) {
      contentEl.innerHTML = '<div class="error">' + escapeHtml(e.message) + '</div>';
      setStatus('Decode threw: ' + e.message);
    }
  }

  function initWasm(cb) {
    if (typeof Go === 'undefined') {
      setStatus('Missing wasm_exec.js. Run: make wasm-exec');
      return;
    }
    setStatus('Loading WASM...');
    const wasmUrl = chrome.runtime.getURL('lib/dump.wasm');
    const go = new Go();
    fetch(wasmUrl)
      .then(r => r.arrayBuffer())
      .then(bytes => WebAssembly.instantiate(bytes, go.importObject))
      .then(({ instance }) => {
        go.run(instance);
        decodeProtobuf = window.decodeProtobuf;
        if (decodeProtobuf) {
          setStatus('WASM ready. Decode a gRPC/Protobuf response or click "Decode last Proto".');
          if (cb) cb();
        } else {
          setStatus('WASM ran but decodeProtobuf not found.');
        }
      })
      .catch(e => {
        setStatus('WASM load failed: ' + e.message);
        contentEl.innerHTML = '<div class="error">' + escapeHtml(e.message) + '</div>';
      });
  }

  searchEl.addEventListener('input', () => {
    const tree = contentEl.querySelector('.tree');
    if (tree && decodeProtobuf) {
      const last = window.__dumpLastDecoded;
      if (last) renderTree(last, searchEl.value);
    }
  });

  decodeLastBtn.addEventListener('click', () => {
    chrome.storage.local.get(['lastProtoPayload', 'lastProtoUrl'], (data) => {
      if (!data.lastProtoPayload || !data.lastProtoPayload.length) {
        setStatus('No intercepted Protobuf response yet. Trigger a gRPC/Protobuf request first.');
        contentEl.innerHTML = '<div class="status">Capture a request with Content-Type: application/grpc-web+proto or application/x-protobuf, then click "Decode last Proto".</div>';
        return;
      }
      const arr = new Uint8Array(data.lastProtoPayload);
      setStatus('Decoding last captured response...');
      decodeBytes(arr);
    });
  });

  loadDemoBtn.addEventListener('click', () => {
    if (!decodeProtobuf) return;
    const demo = new Uint8Array([0x08, 0x2a, 0x12, 0x04, 0x74, 0x65, 0x73, 0x74]);
    decodeBytes(demo);
  });

  window.__dumpNotifyPanelShown = () => {
    if (!decodeProtobuf) initWasm();
  };

  initWasm();
})();
