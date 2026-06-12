---
version: alpha
name: HyperDrop Aurora
website: "https://github.com/treewalk/hyper-drop"
description: "An ethereal dark-canvas file-transfer UI built around \"#080b1a\" (a midnight-navy canvas with a faint indigo cast), frosted-glass surfaces using `backdrop-filter: blur(20px)` at `rgba(255,255,255,0.04)`, and a dual-accent system — primary aurora violet \"#7c6cf0\" and secondary aurora teal \"#4ecdc4\" — joined as a 135° gradient that drifts across brand marks, progress fills, and active-state pills. The system reads as a living, breathing interface: four CSS-animated radial blobs behind all content slowly drift, scale, and fade on 18–25 second cycles, giving every screen an aurora-borealis backdrop without a single static gradient. Display type is set in the system sans at weight 800 with –0.03em tracking. Cards live as glass panels with 16px corners, hairline white borders at 8% opacity, and colored glow shadows on hover. The accent gradient appears on the brand wordmark, circular dropzone rings, sort-pill active states, and progress-bar fills — the only saturated chromatic moments on a page that otherwise trusts translucency, blur, and surface lift for hierarchy."

seo:
  title: "HyperDrop Aurora Design System — Glassmorphism (#080b1a), Aurora Violet (#7c6cf0), 28 components"
  metaDescription: "HyperDrop Aurora's design system as a DESIGN.md file. Deep navy #080b1a canvas, frosted glass surfaces, aurora violet #7c6cf0 and teal #4ecdc4 dual accent, 28 colors, 28 components. For React, Next.js, and AI tools."
  highlights:
    - "Midnight-navy canvas #080b1a with four animated aurora blobs drifting on 18–25s cycles — the page breathes"
    - "Dual-accent gradient linear-gradient(135deg, #7c6cf0, #4ecdc4) — violet-to-teal used on brand, active states, and progress fills"
    - "Frosted-glass surfaces via backdrop-filter: blur(20px) at rgba(255,255,255,0.04) — every card, header, toast, and modal is translucent"
    - "Circular dropzones with rotating conic-gradient rings and orbital glowing dots — the signature interaction moment"
    - "Five-step file-type color language — doc purple, img teal, vid coral, zip amber, file muted — applied consistently across badges, icons, chips, and preview panels"
  tags:
    - "File Transfer"
    - "Productivity & Tools"
    - "Glassmorphism"
  lastUpdated: "2026-06-12"
  author:
    name: "HyperDrop Team"
  opening: |
    HyperDrop Aurora is a glassmorphism-first dark-canvas system for a peer-to-peer file transfer app. The page floor is "#080b1a" — a deep midnight navy with a faint indigo cast that never reads as pure black. Behind every screen, four CSS-animated radial blobs drift, scale, and fade on independent cycles (18s, 22s, 20s, 25s), producing a living aurora-borealis backdrop at 8–15% opacity with a 120px gaussian blur. This atmospheric layer is the system's signature: it gives every glass panel a faintly colored reflection that shifts as the blobs move, making the UI feel alive without any JavaScript animation overhead.

    All interactive surfaces — cards, the header, toast notifications, confirm modals, the variant switcher — use `backdrop-filter: blur(20px)` with `background: rgba(255,255,255,0.04)` and `border: 1px solid rgba(255,255,255,0.08)`. On hover, surfaces lift to 6% opacity and borders strengthen to 14%, with a faint violet glow shadow entering at `box-shadow: 0 0 16px rgba(124,108,240,0.06)`. This three-step translucency ladder (4% → 6% → 12%) carries the entire hierarchy without solid backgrounds or drop shadows.

    The chromatic system is a dual-accent pair: aurora violet "#7c6cf0" and aurora teal "#4ecdc4", joined as `linear-gradient(135deg, #7c6cf0, #4ecdc4)`. This gradient appears on the brand wordmark, active nav pills, sort-pill selected states, progress-bar fills, circular dropzone browse links, and the confirm-modal cancel button. A second violet glow token `rgba(124,108,240,0.2)` and teal glow `rgba(78,205,196,0.15)` extend the pair into box-shadows and background tints. Semantic colors — success teal, danger coral, warning amber — each carry their own glow shadow at 30% opacity.

    This page packages the entire spec into one DESIGN.md file written to the Google Labs spec. Inside: 28 color tokens grouped into canvas, surface, hairline, text, accent, semantic, and aurora-blob families; 12 type tokens running the system sans and monospace families with weight and tracking ranges; a 9-step rounded scale (4px through pill); a 9-step spacing scale on a 4–8px base unit ending at 64px section padding; and 28 component definitions covering glass file cards, circular nebula dropzones, orbital split-panel dropzones, stream strip dropzones, glass table rows, grid preview cards, feed timeline chips, glass toasts, glass confirm modals, and the glass variant switcher.

    Feed the file to Claude, Cursor, or GitHub Copilot and the agent reproduces Aurora's discipline — frosted glass over animated aurora, dual-accent gradient, translucency hierarchy — rather than a generic dark theme. Or reference the tokens directly: every hex, rgba, font, radius, and spacing value is a quoted scalar you can paste into Tailwind config, CSS variables, or your component library. Aurora is worth studying because it proves that glassmorphism, when restrained to 4% surface opacity and 20px blur, scales into a complete file-management UI without sacrificing readability or interaction clarity.
  related:
    - href: "/prototype"
      title: "HyperDrop Prototype Gallery"
      description: "All four visual design directions — Aurora, Paper, Neon, Gradient — with live previews."
    - href: "https://github.com/google-labs-code/design.md"
      title: "The DESIGN.md specification"
      description: "Google Labs' open spec for machine-readable design system files — the format this page is built on."
  questions:
    - id: "primary-color"
      title: "What is Aurora's primary brand color?"
      answer: "Aurora uses a dual-accent system. The primary brand color is aurora violet \"#7c6cf0\" — a medium-saturation blue-violet. It pairs with aurora teal \"#4ecdc4\" as the secondary accent. The two are joined as a 135° gradient `linear-gradient(135deg, #7c6cf0, #4ecdc4)` applied to the brand wordmark, active navigation pills, sort-pill selected states, progress-bar fills, and circular dropzone browse links. Neither color appears alone as a flat background fill — the gradient is the canonical accent expression."
    - id: "glass-surfaces"
      title: "How does Aurora achieve the frosted-glass effect?"
      answer: "Every glass surface uses three properties together: `backdrop-filter: blur(20px)` (with -webkit prefix), `background: rgba(255,255,255,0.04)`, and `border: 1px solid rgba(255,255,255,0.08)`. The 20px blur frosts the aurora blobs and content behind the element. The 4% white overlay lifts the surface just above the canvas. The 8% white border defines the panel edge without a hard line. On hover, the background increases to 6% and the border to 14%, with a violet glow shadow at 6% opacity entering via box-shadow."
    - id: "aurora-blobs"
      title: "What are the aurora blobs and how do they work?"
      answer: "Four absolutely-positioned `div` elements live in a fixed `aurora-bg` container behind all content (z-index: 0). Each blob is a large circle (350–600px) with a radial background color, a 120px CSS `filter: blur()`, and an independent CSS `@keyframes` animation on an 18–25 second cycle. The animations combine `translate()`, `scale()`, and `opacity` changes to make the blobs slowly drift, grow/shrink, and pulse. Colors: blob-1 at `rgba(124,108,240,0.15)` (violet), blob-2 at `rgba(78,205,196,0.10)` (teal), blob-3 at `rgba(200,100,255,0.10)` (magenta), blob-4 at `rgba(124,108,240,0.08)` (violet, dimmer). During drag-over, blob blur tightens from 120px to 80px and opacity multiplies to 1.4, intensifying the background."
    - id: "typography"
      title: "What typography does Aurora use?"
      answer: "Aurora runs the system font stack: `-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif`. The brand wordmark is 1.2rem (19.2px) at weight 800 with –0.03em tracking, rendered as gradient-clipped text. Section headings are 2rem (32px) at weight 800 with –0.03em tracking, also gradient-clipped. Body text runs 0.88rem (14.1px) at weight 500. Metadata — file sizes, transfer speeds, timestamps — uses the monospace stack `'SF Mono', 'Fira Code', 'Cascadia Code', monospace` at 0.75rem (12px) weight 500. Uppercase labels (section headers, sort pills) use 0.72rem at weight 600 with +0.03em to +0.08em tracking."
    - id: "circular-dropzone"
      title: "How does the Nebula dropzone ring animation work?"
      answer: "The Nebula variant (Variant A) renders a 260×260px circular dropzone. A `::before` pseudo-element applies a `conic-gradient` from violet to transparent to teal to transparent, masked to a 2–3px ring shape using `mask: radial-gradient(farthest-side, transparent calc(100% - 3px), #fff calc(100% - 2px))`. This ring rotates on a 12-second reverse linear loop. Inside sits a 236px frosted-glass circle (`backdrop-filter: blur(16px)`) containing the upload icon and browse text. A separate `glow` div renders a `radial-gradient` violet halo behind the entire structure, pulsing on a 3-second cycle. On drag-over, the inner circle's background shifts to `rgba(124,108,240,0.08)`, the border becomes 20% violet, and a 40px glow shadow enters."
    - id: "use-in-project"
      title: "Can I use this DESIGN.md to build a React file-transfer app?"
      answer: "Yes — the file is designed to be fed into Claude, Cursor, or any AI tool that reads structured design tokens. The agent will reproduce Aurora's glass-over-aurora discipline: midnight canvas, four animated blobs, frosted surfaces via backdrop-filter, dual-accent gradient, and translucency hierarchy. Reference the tokens directly inside Tailwind config or CSS variables — every color, type style, radius, and spacing value is a quoted scalar. The 28 component definitions cover file cards, circular dropzones, glass tables, grid preview cards, feed timelines, toasts, modals, and the variant switcher."
    - id: "known-gaps"
      title: "What's missing from this DESIGN.md spec?"
      answer: "A few things: drag-state styling beyond the body overlay and dropzone hover is not exhaustively documented; the simulated upload timer logic (progress ramp, speed calculation, random failure chance) lives in JavaScript and is described functionally rather than tokenized; responsive breakpoints collapse variants but exact mobile layout adjustments are documented as guidelines rather than pixel-precise tokens; and the aurora blob animations use `will-change: transform, opacity` for GPU compositing but the performance implications on low-end devices are not addressed. The file-type color assignments (doc/img/vid/zip/file) are canonical but the extension-to-category mapping logic lives in JS."

colors:
  canvas: "#080b1a"
  surface-glass: "rgba(255,255,255,0.04)"
  surface-glass-hover: "rgba(255,255,255,0.08)"
  surface-glass-active: "rgba(255,255,255,0.12)"
  surface-glass-pressed: "rgba(255,255,255,0.15)"
  surface-solid: "rgba(20,20,40,0.85)"
  surface-solid-heavy: "rgba(20,20,40,0.90)"
  surface-panel: "rgba(255,255,255,0.015)"
  surface-card: "rgba(255,255,255,0.03)"
  hairline: "rgba(255,255,255,0.08)"
  hairline-hover: "rgba(255,255,255,0.14)"
  hairline-active: "rgba(255,255,255,0.20)"
  ink: "#e8e8f8"
  ink-muted: "#7878a0"
  accent-primary: "#7c6cf0"
  accent-secondary: "#4ecdc4"
  accent-gradient: "linear-gradient(135deg, #7c6cf0, #4ecdc4)"
  accent-glow: "rgba(124,108,240,0.20)"
  accent-glow-soft: "rgba(124,108,240,0.06)"
  accent-glow-medium: "rgba(124,108,240,0.10)"
  accent-glow-strong: "rgba(124,108,240,0.15)"
  accent2-glow: "rgba(78,205,196,0.15)"
  accent2-glow-soft: "rgba(78,205,196,0.06)"
  accent2-glow-strong: "rgba(78,205,196,0.12)"
  accent-border: "rgba(124,108,240,0.30)"
  accent2-border: "rgba(78,205,196,0.25)"
  semantic-success: "#4ecdc4"
  semantic-danger: "#ff6b6b"
  semantic-warning: "#ffd93d"
  semantic-success-glow: "rgba(78,205,196,0.30)"
  semantic-danger-glow: "rgba(255,107,107,0.30)"
  semantic-danger-bg: "rgba(255,107,107,0.04)"
  semantic-danger-bg-hover: "rgba(255,107,107,0.10)"
  semantic-danger-border: "rgba(255,107,107,0.25)"
  semantic-warning-glow: "rgba(255,217,61,0.30)"
  file-doc: "#7c6cf0"
  file-doc-bg: "rgba(124,108,240,0.18)"
  file-img: "#4ecdc4"
  file-img-bg: "rgba(78,205,196,0.18)"
  file-vid: "#ff6b6b"
  file-vid-bg: "rgba(255,107,107,0.18)"
  file-zip: "#ffd93d"
  file-zip-bg: "rgba(255,217,61,0.18)"
  file-generic: "#7878a0"
  file-generic-bg: "rgba(120,120,160,0.15)"
  aurora-blob-1: "rgba(124,108,240,0.15)"
  aurora-blob-2: "rgba(78,205,196,0.10)"
  aurora-blob-3: "rgba(200,100,255,0.10)"
  aurora-blob-4: "rgba(124,108,240,0.08)"
  modal-backdrop: "rgba(8,11,26,0.70)"
  switcher-bg: "rgba(16,16,32,0.80)"
  drag-overlay-bg: "rgba(124,108,240,0.06)"
  drag-overlay-border: "rgba(124,108,240,0.40)"

typography:
  brand:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 19.2px
    fontWeight: 800
    lineHeight: 1.20
    letterSpacing: -0.03em
    background: "{colors.accent-gradient}"
    backgroundClip: text
  display:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 32px
    fontWeight: 800
    lineHeight: 1.20
    letterSpacing: -0.03em
    background: "{colors.accent-gradient}"
    backgroundClip: text
  heading-lg:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 22.4px
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: -0.03em
    background: "{colors.accent-gradient}"
    backgroundClip: text
  heading:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 15.2px
    fontWeight: 600
    lineHeight: 1.40
    letterSpacing: 0
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 14.1px
    fontWeight: 500
    lineHeight: 1.50
    letterSpacing: 0
  body-lg:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 15.2px
    fontWeight: 400
    lineHeight: 1.50
    letterSpacing: 0
  meta:
    fontFamily: "'SF Mono', 'Fira Code', 'Cascadia Code', monospace"
    fontSize: 12px
    fontWeight: 500
    lineHeight: 1.40
    letterSpacing: 0
  caption:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 11.5px
    fontWeight: 500
    lineHeight: 1.40
    letterSpacing: 0
  label-uppercase:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 11.5px
    fontWeight: 600
    lineHeight: 1.30
    letterSpacing: 0.04em
    textTransform: uppercase
  label-uppercase-wide:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 13.6px
    fontWeight: 600
    lineHeight: 1.30
    letterSpacing: 0.08em
    textTransform: uppercase
  badge:
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: 9.6px
    fontWeight: 700
    lineHeight: 1.20
    letterSpacing: 0.03em
    textTransform: uppercase

rounded:
  xs: 4px
  sm: 6px
  md: 8px
  lg: 12px
  xl: 16px
  xxl: 20px
  pill: 9999px
  circle: 50%

spacing:
  xxs: 2px
  xs: 4px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
  xxl: 32px
  xxxl: 48px
  section: 64px

effects:
  glass-blur: "backdrop-filter: blur(20px)"
  glass-blur-light: "backdrop-filter: blur(16px)"
  glass-blur-heavy: "backdrop-filter: blur(24px)"
  glass-blur-subtle: "backdrop-filter: blur(12px)"
  glass-blur-soft: "backdrop-filter: blur(8px)"
  aurora-blob-blur: "filter: blur(120px)"
  aurora-blob-blur-active: "filter: blur(80px)"
  glow-primary: "box-shadow: 0 0 16px rgba(124,108,240,0.06)"
  glow-primary-medium: "box-shadow: 0 0 20px rgba(124,108,240,0.10)"
  glow-primary-strong: "box-shadow: 0 0 40px rgba(124,108,240,0.15)"
  glow-secondary-strong: "box-shadow: 0 0 40px rgba(78,205,196,0.12)"
  glow-success: "box-shadow: 0 0 8px rgba(78,205,196,0.30)"
  glow-danger: "box-shadow: 0 0 8px rgba(255,107,107,0.30)"
  glow-warning: "box-shadow: 0 0 8px rgba(255,217,61,0.30)"
  lift-card: "transform: translateY(-2px); box-shadow: 0 8px 32px rgba(0,0,0,0.3), 0 0 16px rgba(124,108,240,0.06)"
  lift-card-strong: "transform: translateY(-3px); box-shadow: 0 12px 40px rgba(0,0,0,0.3), 0 0 20px rgba(124,108,240,0.06)"
  lift-modal: "box-shadow: 0 24px 64px rgba(0,0,0,0.5), 0 0 24px rgba(124,108,240,0.06)"
  lift-toast: "box-shadow: 0 8px 32px rgba(0,0,0,0.4), 0 0 16px rgba(124,108,240,0.06)"
  lift-switcher: "box-shadow: 0 8px 32px rgba(0,0,0,0.5), 0 0 16px rgba(124,108,240,0.05)"

animations:
  duration-fast: "0.15s"
  duration-normal: "0.20s"
  duration-slow: "0.25s"
  duration-slower: "0.30s"
  easing: "ease"
  easing-spring: "cubic-bezier(0.16, 1, 0.3, 1)"
  aurora-drift-1: "18s ease-in-out infinite"
  aurora-drift-2: "22s ease-in-out infinite"
  aurora-drift-3: "20s ease-in-out infinite"
  aurora-drift-4: "25s ease-in-out infinite"
  ring-rotate: "12s linear infinite reverse"
  orbit-primary: "15s linear infinite"
  orbit-secondary: "25s linear infinite reverse"
  glow-pulse: "3s ease-in-out infinite"
  toast-in: "0.3s cubic-bezier(0.16, 1, 0.3, 1)"
  toast-out: "0.2s ease forwards"
  modal-in: "0.25s cubic-bezier(0.16, 1, 0.3, 1)"
  modal-box-in: "0.3s cubic-bezier(0.16, 1, 0.3, 1)"

components:
  header:
    backgroundColor: "rgba(8,11,26,0.70)"
    backdropFilter: "blur(20px)"
    border: "1px solid rgba(255,255,255,0.08)"
    borderBottom: true
    height: 56px
    padding: "0 24px"
    position: "sticky"
  header-nav-link:
    typography: "{typography.body}"
    color: "{colors.ink-muted}"
    padding: "6.4px 13.6px"
    rounded: "{rounded.lg}"
    border: "1px solid transparent"
    transition: "all 0.2s ease"
  header-nav-link-hover:
    color: "{colors.ink}"
    background: "{colors.surface-glass-hover}"
    borderColor: "{colors.hairline-hover}"
  header-nav-link-active:
    color: "{colors.accent-primary}"
    borderColor: "{colors.accent-border}"
    background: "{colors.accent-glow}"
    boxShadow: "{effects.glow-primary-medium}"
  brand-wordmark:
    typography: "{typography.brand}"
    background: "{colors.accent-gradient}"
    backgroundClip: text
    textDecoration: none
  file-card:
    backgroundColor: "{colors.surface-card}"
    backdropFilter: "blur(12px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.xl}"
    padding: "13.6px 16px"
    display: flex
    gap: "{spacing.lg}"
    transition: "all 0.25s ease"
  file-card-hover:
    backgroundColor: "{colors.surface-glass-hover}"
    borderColor: "{colors.hairline-hover}"
    transform: "translateY(-2px)"
    boxShadow: "{effects.lift-card}"
  file-card-error:
    borderColor: "{colors.semantic-danger-border}"
    backgroundColor: "{colors.semantic-danger-bg}"
  file-icon:
    width: 38px
    height: 38px
    rounded: "{rounded.lg}"
    display: flex
    alignItems: center
    justifyContent: center
    typography: "{typography.badge}"
    flexShrink: 0
  file-icon-doc:
    backgroundColor: "{colors.file-doc-bg}"
    color: "{colors.file-doc}"
  file-icon-img:
    backgroundColor: "{colors.file-img-bg}"
    color: "{colors.file-img}"
  file-icon-vid:
    backgroundColor: "{colors.file-vid-bg}"
    color: "{colors.file-vid}"
  file-icon-zip:
    backgroundColor: "{colors.file-zip-bg}"
    color: "{colors.file-zip}"
  file-icon-generic:
    backgroundColor: "{colors.file-generic-bg}"
    color: "{colors.file-generic}"
  progress-bar:
    width: 100%
    height: 4px
    backgroundColor: "rgba(255,255,255,0.06)"
    rounded: "{rounded.xs}"
    marginTop: 6px
    overflow: hidden
  progress-bar-fill:
    height: 100%
    rounded: "{rounded.xs}"
    background: "{colors.accent-gradient}"
    boxShadow: "{effects.glow-primary}"
    transition: "width 0.15s ease"
  progress-bar-fill-done:
    background: "{colors.semantic-success}"
    boxShadow: "{effects.glow-success}"
  progress-bar-fill-error:
    background: "{colors.semantic-danger}"
    boxShadow: "{effects.glow-danger}"
  progress-bar-fill-paused:
    background: "{colors.semantic-warning}"
    boxShadow: "{effects.glow-warning}"
  action-button:
    background: none
    border: none
    color: "{colors.ink-muted}"
    padding: "{spacing.xs}"
    rounded: "{rounded.md}"
    transition: "all 0.2s ease"
  action-button-hover:
    backgroundColor: "rgba(255,255,255,0.06)"
    color: "{colors.ink}"
  action-button-danger-hover:
    color: "{colors.semantic-danger}"
    backgroundColor: "rgba(255,107,107,0.10)"
  search-box:
    backgroundColor: "rgba(255,255,255,0.04)"
    backdropFilter: "blur(16px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.pill}"
    padding: "6.4px 13.6px"
    width: 220px
  search-box-focused:
    borderColor: "rgba(124,108,240,0.40)"
    backgroundColor: "rgba(255,255,255,0.06)"
    boxShadow: "{effects.glow-primary-medium}"
  sort-pill:
    backgroundColor: "{colors.surface-glass}"
    backdropFilter: "blur(12px)"
    border: "1px solid {colors.hairline}"
    color: "{colors.ink-muted}"
    padding: "4.8px 10.4px"
    rounded: "{rounded.pill}"
    typography: "{typography.label-uppercase}"
    transition: "all 0.25s ease"
  sort-pill-active:
    color: "#ffffff"
    borderColor: "rgba(124,108,240,0.40)"
    background: "{colors.accent-gradient}"
    boxShadow: "{effects.glow-primary-medium}"
  nebula-dropzone:
    width: 260px
    height: 260px
    position: relative
    cursor: pointer
    transition: "transform 0.3s ease"
  nebula-dropzone-hover:
    transform: "scale(1.03)"
  nebula-dropzone-dragover:
    transform: "scale(1.06)"
  nebula-inner-circle:
    position: absolute
    inset: 12px
    rounded: "{rounded.circle}"
    backgroundColor: "rgba(255,255,255,0.03)"
    backdropFilter: "blur(16px)"
    border: "1px solid rgba(255,255,255,0.06)"
  nebula-inner-circle-hover:
    backgroundColor: "rgba(124,108,240,0.08)"
    borderColor: "rgba(124,108,240,0.20)"
    boxShadow: "{effects.glow-primary-strong}, inset 0 0 40px rgba(124,108,240,0.05)"
  nebula-ring:
    conicGradient: "from 0deg, rgba(124,108,240,0.40), transparent 30%, rgba(78,205,196,0.30) 60%, transparent 90%"
    mask: "radial-gradient(farthest-side, transparent calc(100% - 3px), #fff calc(100% - 2px))"
    animation: "{animations.ring-rotate}"
  nebula-glow:
    inset: "-20px"
    rounded: "{rounded.circle}"
    background: "radial-gradient(circle, rgba(124,108,240,0.12) 0%, transparent 70%)"
    animation: "{animations.glow-pulse}"
  orbit-dropzone:
    width: 240px
    height: 240px
    position: relative
    cursor: pointer
    transition: "transform 0.3s ease"
  orbit-ring-primary:
    inset: "-10px"
    rounded: "{rounded.circle}"
    border: "1px solid rgba(78,205,196,0.20)"
    animation: "{animations.orbit-primary}"
    dotSize: 6px
    dotColor: "{colors.accent-secondary}"
    dotGlow: "0 0 10px rgba(78,205,196,0.60)"
  orbit-ring-secondary:
    inset: "-24px"
    rounded: "{rounded.circle}"
    border: "1px solid rgba(124,108,240,0.12)"
    animation: "{animations.orbit-secondary}"
    dotSize: 4px
    dotColor: "{colors.accent-primary}"
    dotGlow: "0 0 8px rgba(124,108,240,0.50)"
  stream-dropzone:
    width: 100%
    minHeight: 90px
    border: "2px dashed rgba(255,255,255,0.08)"
    rounded: "{rounded.xl}"
    backgroundColor: "rgba(255,255,255,0.02)"
    backdropFilter: "blur(12px)"
    cursor: pointer
    transition: "all 0.25s ease"
  stream-dropzone-hover:
    borderColor: "rgba(124,108,240,0.30)"
    backgroundColor: "rgba(124,108,240,0.06)"
    boxShadow: "0 0 30px rgba(124,108,240,0.08)"
  table-row:
    borderBottom: "1px solid rgba(255,255,255,0.04)"
    transition: "all 0.2s ease"
  table-row-hover:
    backgroundColor: "rgba(255,255,255,0.04)"
    boxShadow: "{effects.glow-primary}"
  table-row-selected:
    backgroundColor: "rgba(124,108,240,0.08)"
    boxShadow: "inset 0 0 0 1px rgba(124,108,240,0.15)"
  grid-card:
    backgroundColor: "rgba(255,255,255,0.03)"
    backdropFilter: "blur(12px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.xl}"
    overflow: hidden
    transition: "all 0.3s ease"
  grid-card-hover:
    borderColor: "{colors.hairline-hover}"
    transform: "translateY(-3px)"
    boxShadow: "{effects.lift-card-strong}"
  grid-card-selected:
    borderColor: "rgba(124,108,240,0.40)"
    backgroundColor: "rgba(124,108,240,0.06)"
    boxShadow: "0 0 24px rgba(124,108,240,0.10)"
  grid-preview-doc:
    backgroundColor: "rgba(124,108,240,0.06)"
    color: "{colors.file-doc}"
    height: 110px
  grid-preview-img:
    backgroundColor: "rgba(78,205,196,0.06)"
    color: "{colors.file-img}"
    height: 110px
  grid-preview-vid:
    backgroundColor: "rgba(255,107,107,0.06)"
    color: "{colors.file-vid}"
    height: 110px
  grid-preview-zip:
    backgroundColor: "rgba(255,217,61,0.06)"
    color: "{colors.file-zip}"
    height: 110px
  grid-preview-generic:
    backgroundColor: "rgba(120,120,160,0.06)"
    color: "{colors.file-generic}"
    height: 110px
  feed-item:
    display: flex
    gap: "{spacing.lg}"
    padding: "{spacing.lg}"
    rounded: "{rounded.xl}"
    transition: "all 0.25s ease"
  feed-item-hover:
    backgroundColor: "rgba(255,255,255,0.03)"
  feed-avatar:
    width: 36px
    height: 36px
    rounded: "{rounded.circle}"
    backgroundColor: "rgba(255,255,255,0.06)"
    backdropFilter: "blur(8px)"
    border: "1px solid {colors.hairline}"
  feed-chip:
    display: inline-flex
    alignItems: center
    gap: "{spacing.sm}"
    backgroundColor: "rgba(255,255,255,0.03)"
    backdropFilter: "blur(12px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.lg}"
    padding: "{spacing.sm} {spacing.md}"
    transition: "all 0.25s ease"
  feed-chip-hover:
    borderColor: "{colors.hairline-hover}"
    backgroundColor: "rgba(255,255,255,0.05)"
  toast:
    backgroundColor: "rgba(20,20,40,0.85)"
    backdropFilter: "blur(20px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.lg}"
    padding: "11.2px 16px"
    typography: "{typography.body}"
    boxShadow: "{effects.lift-toast}"
    animation: "{animations.toast-in}"
    maxWidth: 340px
  toast-success:
    borderLeft: "3px solid {colors.semantic-success}"
  toast-error:
    borderLeft: "3px solid {colors.semantic-danger}"
  toast-info:
    borderLeft: "3px solid {colors.accent-primary}"
  modal-overlay:
    backgroundColor: "{colors.modal-backdrop}"
    backdropFilter: "blur(8px)"
    animation: "{animations.modal-in}"
  modal-box:
    backgroundColor: "rgba(20,20,40,0.90)"
    backdropFilter: "blur(24px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.xl}"
    padding: "{spacing.xl}"
    maxWidth: 380px
    width: 90%
    boxShadow: "{effects.lift-modal}"
    animation: "{animations.modal-box-in}"
  modal-button-cancel:
    padding: "{spacing.sm} {spacing.xl}"
    rounded: "{rounded.lg}"
    typography: "{typography.body}"
    border: "1px solid {colors.hairline}"
    background: "{colors.surface-glass}"
    color: "{colors.ink}"
  modal-button-danger:
    padding: "{spacing.sm} {spacing.xl}"
    rounded: "{rounded.lg}"
    typography: "{typography.body}"
    background: "rgba(255,107,107,0.15)"
    border: "1px solid rgba(255,107,107,0.30)"
    color: "{colors.semantic-danger}"
  modal-button-danger-hover:
    background: "rgba(255,107,107,0.25)"
    borderColor: "rgba(255,107,107,0.50)"
  variant-switcher:
    backgroundColor: "{colors.switcher-bg}"
    backdropFilter: "blur(20px)"
    border: "1px solid {colors.hairline}"
    rounded: "{rounded.pill}"
    padding: "7.2px 14.4px"
    fontSize: 13.1px
    boxShadow: "{effects.lift-switcher}"
  variant-switcher-button:
    background: none
    border: "1px solid {colors.hairline}"
    color: "{colors.ink}"
    width: 32px
    height: 32px
    rounded: "{rounded.circle}"
    transition: "all 0.2s ease"
  variant-switcher-button-hover:
    background: "{colors.surface-glass-hover}"
    borderColor: "rgba(124,108,240,0.30)"
    boxShadow: "0 0 12px rgba(124,108,240,0.10)"
  scrollbar-thumb:
    width: 6px
    backgroundColor: "rgba(255,255,255,0.08)"
    rounded: 3px
  scrollbar-thumb-hover:
    backgroundColor: "rgba(255,255,255,0.14)"
---

## Overview

HyperDrop Aurora is a glassmorphism-first dark-canvas design system. The canvas is `{colors.canvas}` ("#080b1a") — a deep midnight navy with a faint indigo cast, darker and bluer than generic dark themes. Four CSS-animated radial blobs positioned behind all content drift on independent 18–25 second cycles at `{effects.aurora-blob-blur}` (120px gaussian), producing a living aurora-borealis backdrop at 8–15% opacity.

Every interactive surface uses `{effects.glass-blur}` with `{colors.surface-glass}` (4% white) and `{colors.hairline}` (8% white border). The three-step translucency ladder — `{colors.surface-glass}` (4%) → `{colors.surface-glass-hover}` (6%) → `{colors.surface-glass-active}` (12%) — carries hierarchy without solid fills. Hover adds colored glow shadows at 6% opacity. Cards lift on hover via `translateY(-2px)` with `{effects.lift-card}`.

The dual-accent system — aurora violet `{colors.accent-primary}` ("#7c6cf0") and aurora teal `{colors.accent-secondary}` ("#4ecdc4") — is expressed as `{colors.accent-gradient}` (135° gradient). This gradient appears on the brand wordmark, section headings, active nav pills, sort-pill selected states, progress-bar fills, and circular dropzone browse links. Neither color fills a card background alone.

Typography runs the system sans at weight 800 for display/headings with –0.03em tracking, weight 500 for body, and the monospace stack for file metadata at 12px. The brand wordmark and section headings use gradient-clipped text via `background-clip: text`.

File types carry a five-color language: `{colors.file-doc}` purple, `{colors.file-img}` teal, `{colors.file-vid}` coral, `{colors.file-zip}` amber, `{colors.file-generic}` muted — each with an 18% opacity background tint applied consistently across icon badges, grid preview panels, feed chip icons, and progress-bar states.

**Key Characteristics:**
- **Glassmorphism-first** — every card, header, toast, modal, and switcher uses `{effects.glass-blur}` over the aurora backdrop.
- **Animated aurora canvas** — four blobs at `{effects.aurora-blob-blur}` (120px) drifting on independent cycles; tightens to 80px and intensifies during drag-over.
- **Dual-accent gradient** — `{colors.accent-gradient}` on brand, active states, progress fills; never decorative background fills.
- **Three-step translucency ladder** — 4% → 6% → 12% carries hierarchy; no solid surfaces except the toast/modal backdrop at 85–90%.
- **Circular dropzones** — Nebula variant with rotating conic-gradient ring and pulsing glow halo; Orbit variant with two orbital rings carrying glowing dots.
- **Five-color file-type language** — doc/img/vid/zip/file at 18% background tint, consistent across all view variants.

## Known Gaps

- The four aurora blob positions and animation keyframes are hard-coded in CSS; a tokenized system would need custom properties for `top`, `left`, `width`, `height`, and the transform/opacity keyframe values.
- Drag-over state styling (`body.drag-active`) intensifies blob blur and adds a dashed border overlay — these are CSS-only and not reflected in individual component tokens.
- Simulated upload logic (progress ramp, speed calculation, random failure at 0.1% chance per tick) lives in JavaScript and is not tokenized.
- Responsive breakpoints at 768px collapse split-panel to single-column and hide sort pills/stats — exact mobile adjustments are guidelines rather than token-precise.
- The extension-to-category mapping (e.g., `.pdf` → `doc`, `.mp4` → `vid`) lives in JavaScript, not in the design system.
- `prefers-reduced-motion` is not explicitly handled in the current prototype — aurora blob animations and hover transitions would need `@media (prefers-reduced-motion: reduce)` overrides.
