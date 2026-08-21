import { useEffect } from "react";
export function useShortcuts(handlers: { onCommandPalette?:()=>void; onExecute?:()=>void }) {
  useEffect(()=>{
    const h = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey;
      if (mod && e.key.toLowerCase()==="k") { e.preventDefault(); handlers.onCommandPalette?.(); }
      if (mod && e.key==="Enter") { e.preventDefault(); handlers.onExecute?.(); }
      if (e.key==="Escape") { (document.activeElement as HTMLElement)?.blur(); }
    };
    window.addEventListener("keydown", h);
    return ()=> window.removeEventListener("keydown", h);
  },[handlers]);
}
