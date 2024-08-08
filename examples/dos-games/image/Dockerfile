FROM alpine:latest

WORKDIR /binary
RUN apk add gcc musl-dev && \
    wget -O darkhttpd.c https://raw.githubusercontent.com/emikulic/darkhttpd/master/darkhttpd.c && \
    cc -static -Os -o darkhttpd darkhttpd.c

WORKDIR /site
RUN wget https://js-dos.com/6.22/current/js-dos.js && \
    wget https://js-dos.com/6.22/current/wdosbox.js && \
    wget https://js-dos.com/6.22/current/wdosbox.wasm.js

RUN wget -O doom.zip "https://archive.org/download/DoomsharewareEpisode/doom.ZIP"

RUN wget -O doom.png "https://image.dosgamesarchive.com/screenshots/doom01.png"


COPY index.html .

FROM scratch
COPY --from=0 /site /site
COPY --from=0 /binary /binary

WORKDIR /site
ENTRYPOINT ["/binary/darkhttpd", "/site", "--port", "8000"]

# docker buildx build --push --platform linux/arm64/v8,linux/amd64 --tag ghcr.io/zarf-dev/doom-game:0.0.1 .
