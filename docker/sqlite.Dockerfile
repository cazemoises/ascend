# Sandbox image for SQL challenges: sqlite3 CLI only, no network access is
# needed at runtime because submissions run with --network none. Alpine's
# base image doesn't ship sqlite3, so it's installed here at build time
# (apk add reaches the package mirror during `docker compose build`, which
# is unaffected by the sandbox's --network none — that flag only applies to
# the containers the judge launches to execute submissions).
FROM alpine:3.21

RUN apk add --no-cache sqlite
