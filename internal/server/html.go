package server

import "fmt"

const layoutCSS = `
:root {
  color-scheme: dark;
  --bg: #121a17;
  --panel: #1a2420;
  --label: #24302a;
  --fg: #efe6d4;
  --muted: #9aab9e;
  --tape: #e0943a;
  --tape-ink: #1a140c;
  --sage: #8fbf9a;
  --line: #2e3d36;
  --err: #e07070;
  --perf: #c4b394;
  --mono: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  --display: Bahnschrift, "Arial Narrow", "ui-sans-serif", system-ui, sans-serif;
}
@media (prefers-color-scheme: light) {
  :root {
    color-scheme: light;
    --bg: #efe6d4;
    --panel: #f7f0e2;
    --label: #fff8ec;
    --fg: #1a2420;
    --muted: #5c6b63;
    --tape: #c45e1a;
    --tape-ink: #fff8ec;
    --sage: #2f6b4a;
    --line: #d4c4a8;
    --err: #a33a32;
    --perf: #8a7a60;
  }
}
* { box-sizing: border-box; }
html { background: var(--bg); }
body {
  margin: 0;
  font: 16px/1.45 system-ui, sans-serif;
  background:
    radial-gradient(1200px 400px at 10% -10%, color-mix(in srgb, var(--tape) 12%, transparent), transparent 60%),
    var(--bg);
  color: var(--fg);
}
.skip {
  position: absolute; left: -999px; top: 0;
}
.skip:focus { left: 8px; top: 8px; z-index: 2; padding: 8px 16px; background: var(--panel); }
main { max-width: 56rem; margin: 0 auto; padding: 48px 24px 64px; }
body.gate { min-height: 100vh; display: grid; place-items: center; }
body.gate main { width: min(28rem, calc(100% - 32px)); margin: 0; padding: 0; }
.sheet {
  background: var(--panel);
  padding: 32px;
  border: 1px dashed var(--perf);
}
.eyebrow {
  margin: 0 0 8px;
  font: 550 13px/1.3 var(--mono);
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted);
}
.mark {
  margin: 0;
  font: 700 28px/1 var(--display);
  letter-spacing: 0.14em;
  text-transform: lowercase;
}
.mark::after {
  content: "";
  display: inline-block;
  width: 10px; height: 10px;
  margin-left: 8px;
  background: var(--tape);
  transform: rotate(12deg);
  vertical-align: 2px;
}
h2 { font-size: 18px; font-weight: 650; letter-spacing: -0.02em; margin: 0 0 16px; }
a { color: var(--sage); }
a:focus-visible, button:focus-visible, input:focus-visible {
  outline: 2px solid var(--sage);
  outline-offset: 2px;
}
.muted { color: var(--muted); font-size: 14px; }
.err { color: var(--err); margin: 16px 0; }
label { display: block; margin: 16px 0 0; font-size: 13px; font-weight: 550; }
input[type=text], input[type=password], input[type=file] {
  display: block; width: 100%; margin-top: 8px;
  padding: 12px 16px;
  border: 1px solid var(--line);
  border-radius: 0;
  background: var(--label);
  color: inherit;
  font: inherit;
}
input[type=file] {
  padding: 24px 16px;
  border: 1px dashed var(--perf);
  cursor: pointer;
}
button, .btn {
  display: inline-block;
  margin-top: 16px;
  padding: 12px 20px;
  min-height: 44px;
  border: 0;
  border-radius: 0;
  background: var(--tape);
  color: var(--tape-ink);
  font: 650 16px/1 system-ui, sans-serif;
  cursor: pointer;
  text-decoration: none;
}
button.ghost {
  background: transparent;
  color: var(--fg);
  border: 1px solid var(--line);
}
header.bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 32px;
  padding-bottom: 24px;
  border-bottom: 1px dashed var(--perf);
}
header.bar form { margin: 0; }
header.bar button { margin-top: 0; }
section { margin: 32px 0; }
.manifest { width: 100%; border-collapse: separate; border-spacing: 0 8px; font-variant-numeric: tabular-nums; }
.manifest th {
  text-align: left;
  padding: 0 16px 8px;
  color: var(--muted);
  font: 550 13px/1.3 var(--mono);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border: 0;
}
.manifest td {
  text-align: left;
  padding: 16px;
  background: var(--label);
  border-top: 1px dashed var(--perf);
  border-bottom: 1px dashed var(--perf);
  vertical-align: middle;
}
.manifest td:first-child { border-left: 1px dashed var(--perf); font-family: var(--mono); font-size: 14px; word-break: break-word; }
.manifest td:last-child { border-right: 1px dashed var(--perf); white-space: nowrap; }
.actions { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.actions form { margin: 0; display: inline; }
.actions button { margin-top: 0; padding: 8px 12px; min-height: 40px; }
.stamp {
  display: inline-block;
  font: 600 11px/1 var(--mono);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  padding: 4px 8px;
  border: 1px dashed var(--tape);
  color: var(--tape);
}
.empty { color: var(--muted); margin: 16px 0; padding: 24px 16px; border: 1px dashed var(--line); }
.audit { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
.audit th, .audit td { text-align: left; padding: 12px 8px; border-bottom: 1px solid var(--line); }
.audit th { color: var(--muted); font: 550 13px/1.3 var(--mono); letter-spacing: 0.08em; text-transform: uppercase; }
.audit td:nth-child(4) { font-family: var(--mono); font-size: 14px; word-break: break-word; }
@media (max-width: 640px) {
  main { padding: 24px 16px 48px; }
  .sheet { padding: 24px 16px; }
  .manifest { display: block; overflow-x: auto; }
}
@media (prefers-reduced-motion: reduce) {
  * { transition: none !important; }
}
`

func humanSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KiB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MiB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GiB", float64(n)/(k*k*k))
	}
}

func loginErrorCopy(code string) string {
	switch code {
	case "unauthorized":
		return "Username or password is wrong."
	case "bad_request":
		return "Enter a username and password."
	case "not_ready":
		return "gfs is not ready. Try again in a moment."
	case "":
		return ""
	default:
		return "Sign-in failed. Try again."
	}
}
