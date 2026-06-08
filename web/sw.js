// Offline cache: cache-first for the app shell, network-first for the JSON APIs
// (fresh when online, last-known when offline — including notes you've opened).
const CACHE = 'vulpea-v6';
const SHELL = ['./', 'index.html', 'notes.html', 'bookmarks.html', 'saves.html', 'journal.html', 'tasks.html', 'org.js', 'app.css', 'manifest.json', 'icon.svg'];

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

self.addEventListener('fetch', e => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);

  if (url.pathname.startsWith('/api/')) {
    e.respondWith(
      fetch(req)
        .then(res => { const copy = res.clone(); caches.open(CACHE).then(c => c.put(req, copy)); return res; })
        .catch(() => caches.match(req))
    );
    return;
  }

  e.respondWith(caches.match(req).then(r => r || fetch(req)));
});
