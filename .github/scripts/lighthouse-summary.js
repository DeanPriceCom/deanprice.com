const fs = require('fs');
const path = require('path');

const summaryFile = process.env.GITHUB_STEP_SUMMARY;
const dir = '.lighthouseci';

function writeSummary(markdown) {
  if (summaryFile) {
    fs.appendFileSync(summaryFile, markdown + '\n');
  }
  console.log(markdown);
}

if (!fs.existsSync(dir)) {
  writeSummary('### ⚠️ Lighthouse Audit\nNo audit reports generated. Target site may have been unreachable.');
  process.exit(0);
}

const files = fs.readdirSync(dir).filter(f => f.startsWith('lhr-') && f.endsWith('.json'));

if (files.length === 0) {
  writeSummary('### ⚠️ Lighthouse Audit\nNo JSON reports found in `.lighthouseci`.');
  process.exit(0);
}

const runs = [];
for (const file of files) {
  try {
    const content = fs.readFileSync(path.join(dir, file), 'utf8');
    runs.push(JSON.parse(content));
  } catch (err) {
    console.warn(`[lighthouse-summary] Failed to parse ${file}:`, err);
  }
}

if (runs.length === 0) {
  writeSummary('### ⚠️ Lighthouse Audit\nAll generated reports failed to parse.');
  process.exit(0);
}

function median(arr) {
  const sorted = [...arr].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 !== 0 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

const getScore = (r, cat) => (r.categories?.[cat]?.score ?? 0) * 100;
const getAudit = (r, id) => r.audits?.[id]?.numericValue ?? 0;

const perf = Math.round(median(runs.map(r => getScore(r, 'performance'))));
const a11y = Math.round(median(runs.map(r => getScore(r, 'accessibility'))));
const bp = Math.round(median(runs.map(r => getScore(r, 'best-practices'))));
const seo = Math.round(median(runs.map(r => getScore(r, 'seo'))));

const lcp = median(runs.map(r => getAudit(r, 'largest-contentful-paint'))) / 1000;
const cls = median(runs.map(r => getAudit(r, 'cumulative-layout-shift')));
const tbt = median(runs.map(r => getAudit(r, 'total-blocking-time')));
const fcp = median(runs.map(r => getAudit(r, 'first-contentful-paint'))) / 1000;

const status = (val, target) => val >= target ? '🟢 Pass' : '🔴 Fail';
const statusUnder = (val, max) => val <= max ? '🟢 Pass' : '🔴 Fail';

const markdown = `### 🚀 Mobile Lighthouse & Core Web Vitals Audit

| Category | Score | Target | Status |
| :--- | :---: | :---: | :---: |
| ⚡ **Performance** | **${perf} / 100** | ≥ 90 | ${status(perf, 90)} |
| ♿ **Accessibility** | **${a11y} / 100** | 100 | ${status(a11y, 100)} |
| 🛡️ **Best Practices** | **${bp} / 100** | ≥ 90 | ${status(bp, 90)} |
| 🔍 **SEO** | **${seo} / 100** | ≥ 90 | ${status(seo, 90)} |

#### Core Web Vitals (Median Across ${runs.length} Passes)

| Metric | Measured | Target | Status |
| :--- | :---: | :---: | :---: |
| **Largest Contentful Paint (LCP)** | \`${lcp.toFixed(2)}s\` | ≤ 2.5s | ${statusUnder(lcp, 2.5)} |
| **Cumulative Layout Shift (CLS)** | \`${cls.toFixed(3)}\` | ≤ 0.10 | ${statusUnder(cls, 0.1)} |
| **Total Blocking Time (TBT)** | \`${Math.round(tbt)}ms\` | ≤ 200ms | ${statusUnder(tbt, 200)} |
| **First Contentful Paint (FCP)** | \`${fcp.toFixed(2)}s\` | ≤ 1.8s | ${statusUnder(fcp, 1.8)} |
`;

writeSummary(markdown);
