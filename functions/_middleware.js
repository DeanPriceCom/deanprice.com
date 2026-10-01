class CountryInjector {
  constructor(country) {
    this.country = country;
  }
  element(element) {
    element.prepend(`<meta name="cf-country" content="${this.country}">\n`, { html: true });
  }
}

class RobotsRewriter {
  element(element) {
    element.setAttribute("content", "noindex, nofollow, noarchive, nosnippet");
  }
}

const ALIAS_ROBOTS_TXT = `# Disallow AI models, training scrapers, and web archives
User-agent: GPTBot
User-agent: ChatGPT-User
User-agent: ClaudeBot
User-agent: Claude-User
User-agent: CCBot
User-agent: PerplexityBot
User-agent: Bytespider
User-agent: Amazonbot
User-agent: meta-externalagent
User-agent: cohere-ai
User-agent: YouBot
User-agent: AI2Bot
User-agent: ia_archiver
User-agent: archive.org_bot
User-agent: special_archiver
User-agent: archive.today
Disallow: /

# Allow Search Indexers and Social Bots to fetch root so they read noindex & OG tags
User-agent: Googlebot
User-agent: Bingbot
User-agent: WhatsApp
User-agent: TelegramBot
User-agent: Twitterbot
User-agent: LinkedInBot
User-agent: Applebot
User-agent: facebookexternalhit
User-agent: Facebot
User-agent: Slackbot
User-agent: Slackbot-LinkExpanding
User-agent: Discordbot
Allow: /$
Allow: /index.html
Allow: /favicon.ico
Allow: /*.png
Allow: /*.svg
Disallow: /

# Catch-all for other crawlers
User-agent: *
Disallow: /
`;

const PRIVATE_IPV4_REGEX = /^(?:10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2}|100\.(?:6[4-9]|[7-9]\d|1[01]\d|12[0-7])(?:\.\d{1,3}){2})$/;

export async function onRequest(context) {
  const url = new URL(context.request.url);
  const hostname = url.hostname.toLowerCase();
  const cleanHost = hostname.replace(/^www\./, "");
  const isAliasDomain = cleanHost === "deanprice.uk" ||
                        cleanHost === "deanprice.tr" ||
                        cleanHost === "deanprice.ie" ||
                        cleanHost.endsWith(".pages.dev");

  // Dynamic /robots.txt handling for alias domains vs primary .com
  if (url.pathname === "/robots.txt") {
    if (isAliasDomain) {
      return new Response(ALIAS_ROBOTS_TXT, {
        status: 200,
        headers: {
          "Content-Type": "text/plain; charset=utf-8",
          "Cache-Control": "public, max-age=3600",
          "X-Robots-Tag": "noindex, nofollow, noarchive"
        }
      });
    }
    return await context.next();
  }

  const response = await context.next();

  const contentType = response.headers.get("content-type") || "";
  if (!contentType.includes("text/html")) {
    return response;
  }

  const isDevOrPreview = hostname === "localhost" ||
                         hostname === "127.0.0.1" ||
                         hostname === "::1" ||
                         hostname === "[::1]" ||
                         hostname === "" ||
                         hostname.endsWith(".pages.dev") ||
                         hostname.endsWith(".local") ||
                         hostname.endsWith(".lan") ||
                         hostname.endsWith(".ts.net") ||
                         PRIVATE_IPV4_REGEX.test(hostname);

  let rawCountry = "";
  if (cleanHost === "deanprice.uk" || cleanHost.endsWith(".deanprice.uk")) {
    rawCountry = "GB";
  } else if (cleanHost === "deanprice.tr" || cleanHost.endsWith(".deanprice.tr")) {
    rawCountry = "TR";
  } else if (cleanHost === "deanprice.ie" || cleanHost.endsWith(".deanprice.ie")) {
    rawCountry = "IE";
  } else {
    const queryCountry = isDevOrPreview
      ? url.searchParams.get("country")
      : null;
    rawCountry = queryCountry || context.request.cf?.country || context.request.headers.get("CF-IPCountry") || "";
  }

  const country = /^[A-Za-z0-9]{2}$/.test(rawCountry) ? rawCountry.toUpperCase() : "";

  let rewriter = new HTMLRewriter()
    .on("head", new CountryInjector(country));

  if (isAliasDomain) {
    rewriter = rewriter.on('meta[name="robots"]', new RobotsRewriter());
  }

  const transformed = rewriter.transform(response);

  // Prevent upstream CDN caching of personalized Geo HTML
  const headers = new Headers(transformed.headers);
  headers.set("Cache-Control", "private, no-cache, no-store, must-revalidate");

  if (isAliasDomain) {
    headers.set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet");
  } else {
    const isTestOrError = isDevOrPreview && url.searchParams.has("shield");
    if (isTestOrError) {
      headers.set("X-Robots-Tag", "noindex, nofollow, noarchive");
    }
  }

  return new Response(transformed.body, {
    status: transformed.status,
    statusText: transformed.statusText,
    headers: headers
  });
}

