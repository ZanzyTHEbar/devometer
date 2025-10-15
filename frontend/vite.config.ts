import { defineConfig } from "vite";
import solidPlugin from "vite-plugin-solid";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
    plugins: [solidPlugin(), tailwindcss()],
    server: {
        port: 3000,
        host: true,
        proxy: {
            '/api': {
                target: 'http://localhost:8080',
                changeOrigin: true,
            },
            '/swagger': {
                target: 'http://localhost:8080',
                changeOrigin: true,
            },
        },
    },
    build: {
        target: "esnext",
        sourcemap: true,
        outDir: "dist",
        // Ensure hashed filenames for cache busting
        rollupOptions: {
            output: {
                entryFileNames: 'assets/[name]-[hash].js',
                chunkFileNames: 'assets/[name]-[hash].js',
                assetFileNames: 'assets/[name]-[hash].[ext]',
            },
        },
    },
    optimizeDeps: {
        include: ["solid-js"],
    },
});