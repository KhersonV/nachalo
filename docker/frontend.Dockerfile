FROM node:20-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --no-audit --no-fund

FROM node:20-alpine AS builder
WORKDIR /app
ARG NEXT_PUBLIC_API_BASE
ARG NEXT_PUBLIC_WS_URL
ARG NEXT_PUBLIC_AUTH_BASE
ARG NEXT_PUBLIC_MATCHMAKING_BASE
ENV NEXT_PUBLIC_API_BASE=$NEXT_PUBLIC_API_BASE
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
ENV NEXT_PUBLIC_AUTH_BASE=$NEXT_PUBLIC_AUTH_BASE
ENV NEXT_PUBLIC_MATCHMAKING_BASE=$NEXT_PUBLIC_MATCHMAKING_BASE
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY package*.json ./
COPY next.config.ts ./next.config.ts
COPY tsconfig.json ./tsconfig.json
COPY public ./public
COPY src ./src
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
EXPOSE 3000
CMD ["node", "server.js"]
