import { check } from "@tauri-apps/plugin-updater";
export async function checkUpdate(){ try{ const u=await check(); if(u?.available) await u.downloadAndInstall(); }catch(e){ console.error(e);} }
