export async function onRequest(context) {
  const upstreamUrl = 'https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/uninstall-macos.sh';
  const headers = new Headers();
  const ifNoneMatch = context.request.headers.get('If-None-Match');
  const ifModifiedSince = context.request.headers.get('If-Modified-Since');
  if (ifNoneMatch) headers.set('If-None-Match', ifNoneMatch);
  if (ifModifiedSince) headers.set('If-Modified-Since', ifModifiedSince);

  try {
    const upstream = await fetch(upstreamUrl, {
      headers,
      cf: { cacheEverything: true, cacheTtl: 300 },
    });

    if (upstream.status === 304) {
      return new Response(null, { status: 304 });
    }

    if (upstream.ok) {
      const respHeaders = new Headers(upstream.headers);
      respHeaders.set('Content-Type', 'text/x-shellscript; charset=utf-8');
      respHeaders.set('Cache-Control', 'public, max-age=300, s-maxage=300');
      respHeaders.set('Access-Control-Allow-Origin', '*');
      return new Response(upstream.body, { status: 200, headers: respHeaders });
    }

    return Response.redirect(upstreamUrl, 302);
  } catch (err) {
    return Response.redirect(upstreamUrl, 302);
  }
}
