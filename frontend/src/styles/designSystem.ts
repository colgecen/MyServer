export const colors = {
  bg: "#0a0a12",
  surface: "rgba(18,18,30,0.7)",
  glass: "rgba(255,255,255,0.06)",
  neonCyan: "#00f5ff",
  neonMagenta: "#ff00a8",
  neonViolet: "#7a00ff",
  textPrimary: "#e6e6ff",
  textMuted: "#9aa0b6",
  border: "rgba(120,120,255,0.15)",
} as const;
export const radii = { sm: "8px", md: "12px", lg: "16px", xl: "24px", pill: "999px" } as const;
export const shadows = { glass: "0 8px 32px rgba(0,245,255,0.12), inset 0 1px 0 rgba(255,255,255,0.08)", neon: "0 0 16px rgba(0,245,255,0.6)" } as const;
export const typography = { fontSans: "'Geist Sans','Inter',system-ui,sans-serif", fontMono: "'Geist Mono','JetBrains Mono',monospace", h1: "28px", h2: "20px", body: "14px", caption: "12px" } as const;
