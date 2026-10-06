/**
 * First-Party Error & CSP Function Beacon
 * Endpoint: POST /api/report
 *
 * Receives:
 * 1. W3C Reporting API (application/reports+json)
 * 2. Legacy CSP Reports (application/csp-report)
 * 3. Client & SW Telemetry (application/json or text/plain from navigator.sendBeacon/fetch)
 */

const MAX_PAYLOAD_BYTES = 10 * 1024; // 10 KB

const COLOR_MAP = {
  red: 14427686,    // 0xDC2626 - Uncaught Errors & WASM Panics
  amber: 16096779,  // 0xF59E0B - CSP & Policy Violations
  yellow: 16498468, // 0xFBBF24 - Unhandled Rejections
  gray: 7041664     // 0x6B7280 - Generic / Informational
};

const DEDUPE_TTL_MS = 60 * 1000;
const MAX_CACHE_ENTRIES = 100;
const recentReports = new Map();

function isDuplicateReport(report) {
  const dedupeKey = `${report.type}:${report.url}:${(report.message || "").slice(0, 100)}`;
  const now = Date.now();
  const lastSeen = recentReports.get(dedupeKey);

  if (lastSeen && (now - lastSeen) < DEDUPE_TTL_MS) {
    return true; // Duplicate within 60s cooldown; do not update lastSeen so it re-alerts if error persists
  }

  recentReports.set(dedupeKey, now);
  if (recentReports.size > MAX_CACHE_ENTRIES) {
    const oldestKey = recentReports.keys().next().value;
    if (oldestKey) recentReports.delete(oldestKey);
  }
  return false;
}

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const KNOWN_EXTENSION_POLICIES = /^(?:AGPolicy|goog#html|lit-html|emscripten#workerPolicy|adblock|nordpass|bitwarden|lastpass|react-devtools)/i;

function isFirstPartySource(filename) {
  if (!filename) return false;
  if (filename.startsWith("/")) return true;
  if (!/^https?:\/\//i.test(filename)) return false;

  try {
    const host = new URL(filename).hostname.toLowerCase();
    return (
      host === "deanprice.com" ||
      host.endsWith(".deanprice.com") ||
      host === "deanprice.uk" ||
      host.endsWith(".deanprice.uk") ||
      host === "deanprice.tr" ||
      host.endsWith(".deanprice.tr") ||
      host === "deanprice.ie" ||
      host.endsWith(".deanprice.ie") ||
      host.endsWith(".pages.dev") ||
      host === "localhost" ||
      host === "127.0.0.1" ||
      host === "::1" ||
      host === "[::1]"
    );
  } catch {
    return false;
  }
}

function truncate(str, maxLen) {
  if (!str) return "";
  const s = String(str);
  return s.length > maxLen ? s.slice(0, maxLen - 3) + "..." : s;
}

function isExtensionOrNoise(filename, message, blockedURI, stack, sample, directive) {
  const msgStr = String(message || "");
  if (msgStr === "Script error.") return true;

  const sampleStr = String(sample || "");
  const dirStr = String(directive || "");

  // Note: sampleStr intentionally omitted from targets to prevent content false-positives
  const targets = [
    String(filename || ""),
    msgStr,
    String(blockedURI || ""),
    String(stack || "")
  ];

  for (const s of targets) {
    if (!s) continue;
    if (
      s.includes("extension:") ||
      s.includes("cloudflareinsights.com") ||
      s.includes(":2096") ||
      /denied permission|SecurityError|operation is insecure|access to storage is not allowed|the operation is not supported/i.test(s) ||
      /failed to fetch|networkerror|load failed|aborterror/i.test(s) ||
      (s.includes("wasm_exec.js") && /unexpected token/i.test(s))
    ) {
      return true;
    }
  }

  const isTrustedTypes =
    dirStr.startsWith("trusted-types") ||
    dirStr.startsWith("require-trusted-types-for") ||
    blockedURI === "trusted-types-policy" ||
    blockedURI === "trusted-types-sink";

  if (isTrustedTypes) {
    // 1. Drop known third-party extension policy names (only evaluated for policy creations)
    const isPolicyCreation =
      blockedURI === "trusted-types-policy" ||
      (dirStr.startsWith("trusted-types") && blockedURI !== "trusted-types-sink");

    if (isPolicyCreation && sampleStr) {
      if (KNOWN_EXTENSION_POLICIES.test(sampleStr) || UUID_REGEX.test(sampleStr)) {
        return true;
      }
    }

    // 2. Drop any Trusted Types violation originating from missing, anonymous, or third-party sources
    if (!isFirstPartySource(filename)) {
      return true;
    }
  }

  return false;
}

function normalizeReports(rawJson) {
  const reports = [];

  // 1. W3C Reporting API (Array)
  if (Array.isArray(rawJson)) {
    for (const item of rawJson) {
      if (item && item.type === "csp-violation" && item.body) {
        const body = item.body;
        const sourceFile = body.sourceFile || "";
        const blockedURL = body.blockedURL || "";
        const directive = body.effectiveDirective || body.violatedDirective || "";
        const sample = body.sample || "";

        if (isExtensionOrNoise(sourceFile, "", blockedURL, "", sample, directive)) continue;
        reports.push({
          category: "csp",
          type: "csp_violation",
          url: body.documentURL || "",
          directive: directive,
          blocked: blockedURL,
          sample: sample,
          message: `CSP Violation: ${directive || "directive"} blocked ${blockedURL || "resource"}`,
          details: `Directive: ${directive || ""}\nBlocked URL: ${blockedURL || ""}\nSource: ${sourceFile || ""}:${body.lineNumber || ""}:${body.columnNumber || ""}${sample ? `\nSample: ${sample}` : ""}`,
          severity: "amber"
        });
      }
    }
    return reports;
  }

  // 2. Legacy CSP Report
  if (rawJson && rawJson["csp-report"]) {
    const csp = rawJson["csp-report"];
    const blockedURI = csp["blocked-uri"] || "";
    const sourceFile = csp["source-file"] || csp["source_file"] || "";
    const directive = csp["violated-directive"] || csp["effective-directive"] || "";
    const sample = csp["script-sample"] || csp["sample"] || "";

    if (!isExtensionOrNoise(sourceFile, "", blockedURI, "", sample, directive)) {
      reports.push({
        category: "csp",
        type: "legacy_csp_report",
        url: csp["document-uri"] || "",
        directive: directive,
        blocked: blockedURI,
        sample: sample,
        message: `CSP Violation: ${directive || "directive"} blocked ${blockedURI}`,
        details: `Directive: ${directive || ""}\nBlocked URI: ${blockedURI}\nDocument: ${csp["document-uri"] || ""}${sample ? `\nSample: ${sample}` : ""}`,
        severity: "amber"
      });
    }
    return reports;
  }

  // 3. Custom Telemetry (Runtime errors, WASM traps, Service Worker errors)
  if (rawJson && typeof rawJson === "object") {
    const type = rawJson.type;
    const message = rawJson.message || rawJson.reason || rawJson.detail;
    const filename = rawJson.filename || "";
    const lineno = rawJson.lineno || 0;
    const colno = rawJson.colno || 0;
    const stack = rawJson.stack || "";
    const url = rawJson.url || "/";

    // Drop empty noise or scanner probes lacking any diagnostic details
    if (!message && !stack) return reports;

    const resolvedType = type || "unknown_error";
    const resolvedMessage = message || "Unknown error";

    if (isExtensionOrNoise(filename, resolvedMessage, url, stack)) return reports;
    if (resolvedType === "unknown_error" && resolvedMessage === "Unknown error" && !stack) return reports;
    if (resolvedType === "sw_registration_failure" && /rejected|not supported|security|trustedscripturl|trusted types/i.test(resolvedMessage)) return reports;
    if (resolvedType === "unhandled_rejection" && /^rejected$/i.test(resolvedMessage.trim())) return reports;

    let severity = "red";
    if (resolvedType === "unhandled_rejection") {
      severity = "yellow";
    }

    const loc = filename ? `${filename}:${lineno}:${colno}` : "unknown";
    reports.push({
      category: "runtime",
      type: resolvedType,
      url: url,
      message: resolvedMessage,
      location: loc,
      details: stack ? `Location: ${loc}\n\nStack:\n${stack}` : `Location: ${loc}\nMessage: ${resolvedMessage}`,
      severity: severity
    });
  }

  return reports;
}

async function sendDiscordNotification(webhookUrl, report) {
  if (!webhookUrl) return;

  const color = COLOR_MAP[report.severity] || COLOR_MAP.gray;
  const fields = [
    { name: "Type", value: `\`${truncate(report.type, 100)}\``, inline: true },
    { name: "URL / Path", value: `\`${truncate(report.url || "/", 100)}\``, inline: true }
  ];

  if (report.location) {
    fields.push({ name: "Location", value: `\`${truncate(report.location, 150)}\``, inline: false });
  }
  if (report.directive) {
    fields.push({ name: "Directive", value: `\`${truncate(report.directive, 150)}\``, inline: true });
  }
  if (report.blocked) {
    fields.push({ name: "Blocked Resource", value: `\`${truncate(report.blocked, 200)}\``, inline: false });
  }
  if (report.sample) {
    fields.push({ name: "Sample", value: `\`${truncate(report.sample, 200)}\``, inline: false });
  }

  const detailContent = truncate(report.details || report.message, 1000);
  if (detailContent) {
    fields.push({
      name: "Details",
      value: `\`\`\`\n${detailContent}\n\`\`\``,
      inline: false
    });
  }

  const payload = {
    username: "DeanPrice.com Sentinel",
    embeds: [
      {
        title: `🚨 ${truncate(report.message, 250)}`,
        color: color,
        fields: fields,
        timestamp: new Date().toISOString(),
        footer: {
          text: "deanprice.com zero-cost observability"
        }
      }
    ]
  };

  try {
    const res = await fetch(webhookUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      console.warn(`[report.js] Webhook dispatch returned HTTP ${res.status}: ${res.statusText}`);
    }
  } catch (err) {
    // Prevent unhandled webhook failures from crashing the worker
    console.warn("[report.js] Webhook dispatch error:", err);
  }
}

export async function onRequestPost(context) {
  const request = context.request;

  // 1. Response headers with dynamic X-Robots-Tag (satisfies crawler isolation without touching _headers)
  const responseHeaders = {
    "X-Robots-Tag": "noindex, nofollow, noarchive",
    "Cache-Control": "no-store, no-cache, must-revalidate",
    "Access-Control-Allow-Origin": request.headers.get("origin") || "*",
    "Access-Control-Allow-Methods": "POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type"
  };

  // 2. Non-Standard Port Firewall with Localhost Exemption
  const reqUrl = new URL(request.url);
  const isLocal = reqUrl.hostname === "localhost" || reqUrl.hostname === "127.0.0.1" || reqUrl.hostname === "::1";
  if (!isLocal && reqUrl.port && reqUrl.port !== "80" && reqUrl.port !== "443") {
    return new Response(null, { status: 204, headers: responseHeaders });
  }

  // 3. Content-Length check (guard against payload abuse)
  const contentLength = request.headers.get("content-length");
  if (contentLength && parseInt(contentLength, 10) > MAX_PAYLOAD_BYTES) {
    return new Response("Payload Too Large", {
      status: 413,
      headers: responseHeaders
    });
  }

  // 4. Drop automated scraper spam if detected by Cloudflare bot management
  const botScore = request.cf?.botManagement?.score;
  if (typeof botScore === "number" && botScore < 30) {
    return new Response(null, { status: 204, headers: responseHeaders });
  }

  let rawBody = "";
  try {
    rawBody = await request.text();
  } catch (_) {
    return new Response(null, { status: 204, headers: responseHeaders });
  }

  if (!rawBody || !rawBody.trim()) {
    return new Response(null, { status: 204, headers: responseHeaders });
  }

  let parsed = null;
  try {
    parsed = JSON.parse(rawBody);
  } catch (_) {
    // Non-JSON or malformed payload; ignore safely
    return new Response(null, { status: 204, headers: responseHeaders });
  }

  const reports = normalizeReports(parsed);
  const webhookUrl = context.env.DISCORD_WEBHOOK_URL || context.env.ALERT_WEBHOOK_URL;

  for (const report of reports) {
    // Stream structured JSON to Cloudflare Real-Time Tail Logs
    console.error(`[telemetry] ${JSON.stringify(report)}`);

    // Asynchronously dispatch Discord alert if configured and not duplicate
    if (webhookUrl && context.waitUntil && !isDuplicateReport(report)) {
      context.waitUntil(sendDiscordNotification(webhookUrl, report));
    }
  }

  return new Response(null, {
    status: 204,
    headers: responseHeaders
  });
}

export async function onRequestOptions(context) {
  return new Response(null, {
    status: 204,
    headers: {
      "Access-Control-Allow-Origin": context.request.headers.get("origin") || "*",
      "Access-Control-Allow-Methods": "POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
      "Access-Control-Max-Age": "86400",
      "X-Robots-Tag": "noindex, nofollow, noarchive",
      "Cache-Control": "no-store, no-cache, must-revalidate"
    }
  });
}
