/**
 * The browser's way to the draw service. A server route, not a routeRules
 * proxy, because those fix their target at build time and the service's
 * address is only known where it is deployed.
 */
export default defineEventHandler((event) => {
  const { apiInternal } = useRuntimeConfig(event)
  const path = event.path.replace(/^\/api/, '')
  return proxyRequest(event, `${String(apiInternal).replace(/\/$/, '')}${path}`)
})
