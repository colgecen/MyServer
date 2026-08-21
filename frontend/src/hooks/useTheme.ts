import { useEffect, useState } from "react";
import { getTheme, setTheme, Theme } from "../lib/theme";
export function useTheme(){
  const [theme,setThemeState]=useState<Theme>(getTheme());
  useEffect(()=>{ document.documentElement.setAttribute("data-theme",theme); },[theme]);
  const toggle=()=>{ const n = theme==="dark"?"light":"dark" as Theme; setTheme(n); setThemeState(n); };
  return { theme, toggle };
}
