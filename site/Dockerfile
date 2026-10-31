# Builds the documentation and landing site at quorum.olawalelawal.dev.
# Build context is the repository root so the console screenshots can be shared.
FROM --platform=$BUILDPLATFORM node:26-alpine AS build

WORKDIR /app/site
COPY site/package.json site/package-lock.json ./
RUN npm ci
COPY site/ ./
RUN npm run build

FROM caddy:2-alpine

COPY site/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/site/dist /srv

EXPOSE 80
