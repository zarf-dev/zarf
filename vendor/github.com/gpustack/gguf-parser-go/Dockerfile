FROM scratch
ARG TARGETOS
ARG TARGETARCH
COPY --chmod=755 .dist/gguf-parser-${TARGETOS}-${TARGETARCH} /bin/gguf-parser
ENTRYPOINT ["/bin/gguf-parser"]
