// Shared clipboard helper, loaded by every page that needs to copy a string
// (Files, Shares). navigator.clipboard is undefined outside a secure context,
// so on the plain-HTTP LAN (http://<lan-ip>) it falls back to a hidden
// textarea + execCommand('copy') — deprecated but the only option that works
// there. Returns whether the copy succeeded.
async function hdCopy(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try { await navigator.clipboard.writeText(text); return true; } catch (e) { /* fall through */ }
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.top = '-1000px';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch (e) {
    return false;
  }
}
