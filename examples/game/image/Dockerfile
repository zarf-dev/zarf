FROM alpine:latest

ARG GAME_URL
ARG GAME_ARGS

WORKDIR /site
RUN wget https://js-dos.com/6.22/current/js-dos.js && \
    wget https://js-dos.com/6.22/current/wdosbox.js && \
    wget https://js-dos.com/6.22/current/wdosbox.wasm.js && \
    wget -O game.zip "$GAME_URL"

COPY index.html .
RUN sed -i s/GAME_ARGS/$GAME_ARGS/ index.html

WORKDIR /binary
RUN apk add gcc musl-dev && \
    wget -O darkhttpd.c https://raw.githubusercontent.com/emikulic/darkhttpd/master/darkhttpd.c && \
    cc -static -Os -o darkhttpd darkhttpd.c

FROM scratch
COPY --from=0 /site /site
COPY --from=0 /binary /binary

WORKDIR /site
ENTRYPOINT ["/binary/darkhttpd", "/site", "--port", "8000"]

#  docker buildx build --push --platform linux/arm/v7,linux/arm64/v8,linux/amd64 --tag defenseunicorns/zarf-game:doom --build-arg GAME_URL=https://archive.org/download/DoomsharewareEpisode/doom.ZIP --build-arg GAME_ARGS=\"DOOM.EXE\" .
