#!/zarf-bin/busybox sh
set -ex

RUN=/zarf-bin/busybox

# Before running anything, verify the busybox binary 
RUN echo "$SHA256_BUSYBOX  /zarf-bin/busybox" | RUN sha256sum -c

# Wait to receive files piped via tar/netcat
RUN netcat -l -p 25000 -w 360 | RUN tar xv

# Verify that the zarf assets are properly loaded
RUN echo "$SHA256_ZARF zarf" | RUN sha256sum -c
RUN echo "$SHA256_IMAGES seed-images.tar" | RUN sha256sum -c

# Load the seed registry
/payload/zarf init bootstrap /payload/seed-images.tar library/registry:2.7.1 -l=trace


