import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8888",
        changeOrigin: true,
      },
      "/media": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8888",
        changeOrigin: true,
      },
      "/profile-media": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8888",
        changeOrigin: true,
      },
    },
  },
});
