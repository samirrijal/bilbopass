// workers/stops-search.js
// Cloudflare Worker for edge-cached stop search using D1

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, {
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, OPTIONS',
          'Access-Control-Allow-Headers': 'Content-Type',
        },
      });
    }

    const lat = parseFloat(url.searchParams.get('lat'));
    const lon = parseFloat(url.searchParams.get('lon'));
    const radius = parseFloat(url.searchParams.get('radius') || '0.5'); // km

    if (isNaN(lat) || isNaN(lon)) {
      return new Response(JSON.stringify({ error: 'lat and lon are required' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    // Try cache first
    const cache = caches.default;
    const cacheKey = new Request(url.toString());
    let response = await cache.match(cacheKey);

    if (response) {
      return response;
    }

    // Query D1 (Cloudflare's edge SQLite)
    const results = await env.DB.prepare(`
      SELECT
        id,
        name,
        lat,
        lon,
        (
          6371 * acos(
            cos(radians(?1)) * cos(radians(lat)) *
            cos(radians(lon) - radians(?2)) +
            sin(radians(?1)) * sin(radians(lat))
          )
        ) AS distance
      FROM stops
      WHERE distance < ?3
      ORDER BY distance
      LIMIT 20
    `).bind(lat, lon, radius).all();

    response = new Response(JSON.stringify(results.results || []), {
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'public, max-age=300',
        'Access-Control-Allow-Origin': '*',
      },
    });

    // Cache for 5 minutes
    await cache.put(cacheKey, response.clone());

    return response;
  },
};
