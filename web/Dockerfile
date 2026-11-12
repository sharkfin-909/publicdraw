FROM node:22-alpine AS build
RUN corepack enable && corepack prepare pnpm@11.23.0 --activate
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM node:22-alpine
WORKDIR /app
COPY --from=build /src/.output ./
USER node
ENV HOST=0.0.0.0 PORT=3000
CMD ["node", "server/index.mjs"]
