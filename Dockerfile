FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH

USER nonroot:nonroot
 
COPY --chown=nonroot:nonroot "build/zarf-linux-$TARGETARCH" /zarf

CMD ["/zarf", "internal", "agent", "-l=trace", "--no-log-file"]
