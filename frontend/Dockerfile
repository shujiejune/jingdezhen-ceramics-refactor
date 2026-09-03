# --- Stage 1: Build the TanStack Start SSR bundle ---
FROM node:22-alpine AS builder

WORKDIR /app

# Enable pnpm via corepack
RUN corepack enable && corepack prepare pnpm@11.19.0 --activate

# Copy lockfile + package.json first (layer caching)
COPY package.json pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy source
COPY . .

# Build the SSR bundle (outputs to .output/)
RUN pnpm build

# --- Stage 2: Production runtime ---
FROM node:22-alpine AS production

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@11.19.0 --activate

# Copy the built output + package.json (for start script)
COPY --from=builder /app/.output ./.output
COPY --from=builder /app/package.json ./

EXPOSE 3000

# Start the SSR server (node .output/server/index.mjs)
CMD ["pnpm", "start"]
