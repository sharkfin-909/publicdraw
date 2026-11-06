export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: false },
  ssr: true,

  runtimeConfig: {
    // Set where it is deployed by NUXT_API_INTERNAL and NUXT_PUBLIC_SITE_URL.
    // Server-side only. Pages render on the server, so they cannot go through
    // the browser-facing /api proxy — they talk to the service directly.
    apiInternal: 'http://localhost:8080/v1',

    public: {
      // Absolute origin, needed for canonicals, Open Graph and the sitemap.
      siteUrl: 'http://localhost:3000',
      siteName: 'publicdraw',

      // The draw service, proxied by server/api so the browser can talk to it
      // without the Go server needing CORS headers.
      apiBase: '/api',
      // Fetched by the browser directly, never through our own server: the
      // point of client-side verification is that the beacon does not reach
      // you via the party whose draw you are checking.
      drandBase: 'https://api.drand.sh',
    },
  },

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        // The seal, so a mobile browser's chrome joins the record side of the
        // palette rather than picking its own colour.
        { name: 'theme-color', content: '#0a5f72' },
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Newsreader:opsz,wght@6..72,400;6..72,600&family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:wght@400;500;600&display=swap',
        },
      ],
    },
  },
})
