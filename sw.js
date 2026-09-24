const STATIC_CACHE = 'dean-price-vault-CACHE_VERSION_PLACEHOLDER';

const ASSETS_TO_CACHE = [
  '/',
  '/manifest.json',
  '/wasm_exec.js',
  '/main.wasm',
  '/icon-192.png',
  '/icon-512.png',
  '/favicon.ico',
  '/favicon.svg'
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => {
      console.log('[Service Worker] Caching core assets');
      return cache.addAll(ASSETS_TO_CACHE);
    }).then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (cacheName !== STATIC_CACHE) {
            console.log('[Service Worker] Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;

  if (!event.request.url.startsWith(self.location.origin)) return;

  const url = new URL(event.request.url);

  // Navigation requests (HTML document): Network-First with tight timeout fallback
  if (event.request.mode === 'navigate' || url.pathname === '/' || url.pathname === '/index.html') {
    const TIMEOUT_MS = 1200;

    event.respondWith(
      (async () => {
        const cache = await caches.open(STATIC_CACHE);

        const timeoutPromise = new Promise((resolve) => setTimeout(resolve, TIMEOUT_MS));
        const fetchPromise = fetch(event.request).then(async (response) => {
          if (response && response.ok) {
            await cache.put(event.request, response.clone());
          }
          return response;
        }).catch(() => null);

        const networkResponse = await Promise.race([fetchPromise, timeoutPromise.then(() => null)]);

        if (networkResponse) {
          return networkResponse;
        }

        // Offline or network timeout -> serve cached version (ignoring query strings like ?utm_source=pwa)
        const cached = await cache.match(event.request, { ignoreSearch: true }) || await cache.match('/');
        if (cached) {
          return cached;
        }

        // Final fallback if not in cache yet
        return (await fetchPromise) || Response.error();
      })()
    );
    return;
  }

  // Static Assets: Cache-First
  event.respondWith(
    caches.match(event.request).then((cachedResponse) => {
      return cachedResponse || fetch(event.request);
    })
  );
});