import { defineConfig } from "@playwright/test";
export default defineConfig({ testDir:"./tests", webServer:{ command:"npm run dev", url:"http://localhost:1420", reuseExistingServer:true }});
