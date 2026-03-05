/**
 * DUMP Protobuf DevTools: create panel and intercept gRPC/Protobuf responses.
 */
const PROTO_CONTENT_TYPES = [
  'application/grpc-web+proto',
  'application/grpc-web',
  'application/x-protobuf',
  'application/protobuf',
  'application/vnd.google.protobuf'
];

chrome.devtools.panels.create(
  'DUMP Proto',
  '',
  'panel.html',
  (panel) => {
    panel.onShown.addListener((window) => {
      if (window && window.__dumpNotifyPanelShown) {
        window.__dumpNotifyPanelShown();
      }
    });
  }
);

function getContentType(request) {
  const h = request.response && request.response.headers;
  if (!h) return '';
  if (typeof h['Content-Type'] === 'string') return h['Content-Type'];
  if (Array.isArray(h)) {
    const ct = h.find(x => (x.name || x).toString().toLowerCase() === 'content-type');
    return ct ? (ct.value || ct) : '';
  }
  return '';
}

chrome.devtools.network.onRequestFinished.addListener((request) => {
  const ct = getContentType(request).toLowerCase();
  const isProto = PROTO_CONTENT_TYPES.some(t => ct.includes(t));
  if (isProto && request.getContent) {
    request.getContent((content, encoding) => {
      if (content && encoding === 'base64') {
        try {
          const binary = Uint8Array.from(atob(content), c => c.charCodeAt(0));
          chrome.storage.local.set({
            lastProtoUrl: request.url,
            lastProtoPayload: Array.from(binary),
            lastProtoTime: Date.now()
          });
        } catch (_) {}
      }
    });
  }
});
