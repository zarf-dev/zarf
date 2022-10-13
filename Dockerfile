FROM ubuntu:latest
RUN useradd -mu 1000 zarf
RUN mkdir /home/zarf/.config

FROM scratch
ARG TARGETARCH

ADD "build/zarf-linux-$TARGETARCH" /zarf
EXPOSE 8443

COPY --from=0 /etc/passwd /etc/passwd
COPY --from=0 /home/zarf/.config /home/zarf/.config

ENV USER=zarf
USER zarf

CMD ["/zarf", "internal", "agent", "-l=trace"]
