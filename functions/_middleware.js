class CountryInjector {
  constructor(country) {
    this.country = country;
  }
  element(element) {
    element.prepend(`<meta name="cf-country" content="${this.country}">\n`, { html: true });
  }
}

export async function onRequest(context) {
  const response = await context.next();

  const contentType = response.headers.get("content-type") || "";
  if (!contentType.includes("text/html")) {
    return response;
  }

  const url = new URL(context.request.url);
  const hostname = url.hostname.toLowerCase();
  const isDevOrPreview = hostname === "localhost" ||
                         hostname === "127.0.0.1" ||
                         hostname === "::1" ||
                         hostname === "[::1]" ||
                         hostname === "" ||
                         hostname.endsWith(".pages.dev") ||
                         hostname.endsWith(".local") ||
                         hostname.endsWith(".lan") ||
                         hostname.endsWith(".ts.net") ||
                         hostname.startsWith("192.168.") ||
                         hostname.startsWith("10.") ||
                         hostname.startsWith("172.") ||
                         hostname.startsWith("100.");

  const queryCountry = isDevOrPreview
    ? url.searchParams.get("country")
    : null;
  const rawCountry = queryCountry || context.request.cf?.country || context.request.headers.get("CF-IPCountry") || "";
  const country = /^[A-Za-z0-9]{2}$/.test(rawCountry) ? rawCountry.toUpperCase() : "";

  const transformed = new HTMLRewriter()
    .on("head", new CountryInjector(country))
    .transform(response);

  // Prevent upstream CDN caching of personalized Geo HTML
  const headers = new Headers(transformed.headers);
  headers.set("Cache-Control", "private, no-cache, no-store, must-revalidate");

  const isTestOrError = isDevOrPreview && url.searchParams.has("shield");
  if (isTestOrError) {
    headers.set("X-Robots-Tag", "noindex, nofollow, noarchive");
  }

  return new Response(transformed.body, {
    status: transformed.status,
    statusText: transformed.statusText,
    headers: headers
  });
}
