FROM scratch
ARG TARGETARCH

ADD "build/zarf-linux-$TARGETARCH" /zarf
EXPOSE 8443

ENV USER=zarf

CMD ["/zarf", "internal", "agent", "-l=trace"]