import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

// Cible du backend Go. En local : http://localhost:8080 (défaut). Pour pointer
// un backend hébergé sur un autre PC du réseau, définir VITE_API_TARGET, par ex.
// dans frontend/.env.local : VITE_API_TARGET=http://192.168.1.5:8080
// loadEnv lit les fichiers .env/.env.local (process.env seul ne les voit pas).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = env.VITE_API_TARGET || "http://localhost:8080";
  return {
    plugins: [react()],
    server: { proxy: { "/api": apiTarget } },
    build: { chunkSizeWarningLimit: 1100 },
    optimizeDeps: { exclude: ["maplibre-gl"] },
  };
});
