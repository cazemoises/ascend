FROM node:24-alpine AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/src/lib/liveRoomConnection.ts ./src/lib/liveRoomConnection.ts
COPY web/tests ./tests
RUN npm test

FROM golang:1.26-alpine AS backend
WORKDIR /build
COPY go.work go.work.sum* go.mod go.sum* ./
COPY api ./api
COPY judge/go.mod judge/go.sum ./judge/
COPY pkg/go.mod ./pkg/
RUN cd api && CGO_ENABLED=0 go test -c -tags integration ./internal/handler -o /bin/live-room-tests

FROM node:24-alpine
WORKDIR /build
COPY --from=backend /bin/live-room-tests /bin/live-room-tests
COPY --from=frontend /web/package.json ./web/package.json
COPY --from=frontend /web/src ./web/src
COPY --from=frontend /web/tests ./web/tests
COPY --from=frontend /web/node_modules/yjs ./web/node_modules/yjs
COPY --from=frontend /web/node_modules/lib0 ./web/node_modules/lib0
COPY --from=frontend /web/node_modules/isomorphic.js ./web/node_modules/isomorphic.js
COPY --from=frontend /web/node_modules/ws ./web/node_modules/ws
ENTRYPOINT ["/bin/live-room-tests", "-test.run=TestLiveRoomYjsConvergence", "-test.timeout=90s", "-test.v"]
