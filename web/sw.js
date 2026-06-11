// Caching strategy:
//   - page navigations: network-first (fresh after every deploy) + redirect-safe
//     (a redirected response can't satisfy a navigation request, which previously
//     broke the Contacts tab via the /index.html -> / redirect), cached shell as
//     the offline fallback.
//   - JSON APIs: network-first, last-known cache when offline.
//   - static assets (css/js/icons): cache-first.
// The server (handleServiceWorker in main.go) rewrites this token to a content
// hash of the embedded web assets when serving sw.js, so this literal is only a
// fallback — every deploy that changes an asset auto-busts the cache, no manual
// bump needed.
const CACHE = 'vulpea-v8';
// NB: precache './' (the canonical start URL), NOT 'index.html' — the latter
// 301-redirects to '/', and a redirected response cannot be cached/served safely.
const SHELL = ['./', 'notes.html', 'bookmarks.html', 'saves.html', 'feeds.html', 'journal.html', 'tasks.html', 'org.js', 'app.css', 'manifest.json', 'icon.svg'];

self.addEventListener('install', e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

// Rebuild a redirected response as a plain one so it can satisfy a navigation.
async function unredirect(res) {
  if (!res || !res.redirected) return res;
  const body = await res.blob();
  return new Response(body, { status: 200, statusText: 'OK', headers: res.headers });
}

self.addEventListener('fetch', e => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);

  // JSON APIs: network-first, fall back to last-known cache.
  if (url.pathname.startsWith('/api/')) {
    e.respondWith(
      fetch(req)
        .then(res => { const copy = res.clone(); caches.open(CACHE).then(c => c.put(req, copy)); return res; })
        .catch(() => caches.match(req))
    );
    return;
  }

  // Page navigations: network-first + redirect-safe, cached shell when offline.
  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req)
        .then(unredirect)
        .catch(() => caches.match(req).then(r => r || caches.match('./')))
    );
    return;
  }

  // Static assets: cache-first.
  e.respondWith(caches.match(req).then(r => r || fetch(req)));
});
