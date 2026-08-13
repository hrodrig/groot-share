package server

const layoutCSS = `
:root { color-scheme: light dark; --bg: #0f1419; --fg: #e7ecf1; --muted: #8b9aab; --acc: #3d8bfd; --line: #243044; --err: #f87171; }
@media (prefers-color-scheme: light) {
  :root { --bg: #f6f7f9; --fg: #12151a; --muted: #5c6b7a; --acc: #1d4ed8; --line: #d5dbe3; --err: #b91c1c; }
}
* { box-sizing: border-box; }
body { margin: 0; font: 16px/1.45 system-ui, sans-serif; background: var(--bg); color: var(--fg); }
main { max-width: 52rem; margin: 2rem auto; padding: 0 1.25rem; }
h1 { font-size: 1.35rem; font-weight: 650; letter-spacing: -0.02em; }
a { color: var(--acc); }
.muted { color: var(--muted); font-size: 0.9rem; }
.err { color: var(--err); }
label { display: block; margin: 0.75rem 0; }
input[type=text], input[type=password], input[type=file] { width: 100%; max-width: 24rem; padding: 0.45rem 0.55rem; border: 1px solid var(--line); border-radius: 6px; background: transparent; color: inherit; }
button, .btn { display: inline-block; margin-top: 0.5rem; padding: 0.4rem 0.85rem; border: 0; border-radius: 6px; background: var(--acc); color: #fff; font: inherit; cursor: pointer; text-decoration: none; }
button.ghost { background: transparent; color: var(--fg); border: 1px solid var(--line); }
header.bar { display: flex; justify-content: space-between; align-items: center; gap: 1rem; margin-bottom: 1.5rem; }
table { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
th, td { text-align: left; padding: 0.45rem 0.35rem; border-bottom: 1px solid var(--line); }
th { color: var(--muted); font-weight: 550; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; }
.empty { color: var(--muted); margin: 1.5rem 0; }
`
