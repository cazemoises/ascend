# Sandbox image for C/C++ challenges: gcc/g++ only, no network access is
# needed at runtime because submissions run with --network none. There's no
# official minimal Alpine image that ships both compilers pre-installed (the
# official `gcc` image is the much larger buildpack-deps/Debian one), so —
# same reasoning as sqlite.Dockerfile next to this file — they're installed
# here at build time (apk add reaches the package mirror during `docker
# compose build`, unaffected by the sandbox's --network none, which only
# applies to the containers the judge launches to execute submissions).
FROM alpine:3.21

RUN apk add --no-cache gcc g++ musl-dev
