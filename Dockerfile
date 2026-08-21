FROM node:24-alpine AS web-build
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server/ ./server/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/army-chess ./server/cmd/army-chess

FROM alpine:3.22
RUN apk add --no-cache ca-certificates wget
RUN addgroup -S army && adduser -S -G army army
WORKDIR /app
COPY --from=server-build /out/army-chess /app/army-chess
COPY --from=web-build /src/web/dist /app/web/dist
USER army
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/army-chess"]
