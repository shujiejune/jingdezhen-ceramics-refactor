import { defineConfig } from "@solidjs/start/config";
import { tanstackRouter } from "@tanstack/router-plugin/vite";

export default defineConfig({
  // SolidStart-specific options would go here at the top level.
  ssr: true,
  server: {
    preset: "bun",
  },
  // All standard Vite configuration must be placed inside this `vite` object.
  vite: {
    plugins: [
      tanstackRouter({
        routesDirectory: "./src/routes",
        generatedRouteTree: "./src/routeTree.gen.ts",
        target: "solid",
      }),
      // The solid() plugin is automatically included by SolidStart's
      // defineConfig, so you don't need to add it here manually.
    ],
  },
});
