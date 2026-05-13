/// <reference lib="webworker" />

import { cleanupOutdatedCaches, precacheAndRoute, matchPrecache } from 'workbox-precaching'
import { registerRoute, setCatchHandler } from 'workbox-routing'
import { CacheFirst, NetworkFirst, NetworkOnly, StaleWhileRevalidate } from 'workbox-strategies'
import { ExpirationPlugin } from 'workbox-expiration'
import { CacheableResponsePlugin } from 'workbox-cacheable-response'
import { clientsClaim } from 'workbox-core'

declare let self: ServiceWorkerGlobalScope & { __WB_MANIFEST: Array<unknown> }

clientsClaim()
cleanupOutdatedCaches()
precacheAndRoute(self.__WB_MANIFEST)

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }
})

// HTML documents: NetworkFirst
registerRoute(
  ({ request }) => request.destination === 'document',
  new NetworkFirst({
    cacheName: 'html-cache',
    networkTimeoutSeconds: 3,
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 * 30 })
    ]
  })
)

// JS/CSS/Worker: StaleWhileRevalidate
registerRoute(
  ({ request }) => ['script', 'style', 'worker'].includes(request.destination),
  new StaleWhileRevalidate({
    cacheName: 'static-resources',
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 * 30 })
    ]
  })
)

// Images & Fonts: CacheFirst
registerRoute(
  ({ request }) => ['image', 'font'].includes(request.destination),
  new CacheFirst({
    cacheName: 'static-assets',
    plugins: [
      new CacheableResponsePlugin({ statuses: [0, 200] }),
      new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 * 30 })
    ]
  })
)

// API: always hit the local Go backend. Cached API responses can hide the fact
// that the backend is not running, especially when launching an installed PWA.
registerRoute(
  ({ url }) => url.pathname === '/api' || url.pathname.startsWith('/api/'),
  new NetworkOnly()
)

// Offline fallback
setCatchHandler(async ({ event }) => {
  if (!('request' in event)) {
    return Response.error()
  }
  const request = (event as FetchEvent).request
  if (request.destination === 'document') {
    const fallback = await matchPrecache('/offline.html')
    return fallback || Response.error()
  }
  if (request.destination === 'image') {
    const fallback = await matchPrecache('/offline-image.svg')
    return fallback || Response.error()
  }
  return Response.error()
})
