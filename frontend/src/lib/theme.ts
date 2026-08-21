export type Theme = "dark" | "light";
const KEY = "myserver-theme";
export function getTheme(): Theme { return (localStorage.getItem(KEY) as Theme) || "dark"; }
export function setTheme(t: Theme){ localStorage.setItem(KEY,t); document.documentElement.setAttribute("data-theme",t); }
export function initTheme(){ setTheme(getTheme()); }
