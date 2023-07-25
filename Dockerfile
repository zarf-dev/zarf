FROM cgr.dev/chainguard/static:latest
ARG TARGETARCH

USER 65532:65532

COPY --chown=nonroot:nonroot "build/zarf-linux-$TARGETARCH" /zarf

CMD ["/zarf", "internal", "agent", "-l=trace", "--no-log-file"]
