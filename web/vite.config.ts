import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// In development the Go server runs on :7700 (make dev-api); Vite proxies the API.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: { "/api": { target: "http://127.0.0.1:7700", changeOrigin: false } },
  },
  build: { outDir: "dist", emptyOutDir: false },
});
