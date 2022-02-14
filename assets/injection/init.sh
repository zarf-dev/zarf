#!/zarf-bin/busybox sh
set -ex

# Before /zarf-bin/busyboxning anything, verify the busybox binary 
/zarf-bin/busybox echo "$SHA256_BUSYBOX  /zarf-bin/busybox" | /zarf-bin/busybox sha256sum -c

# Wait to receive files piped via tar/netcat
/zarf-bin/busybox netcat -l -p 25000 -w 360 | /zarf-bin/busybox tar xv

# Verify that the zarf assets are properly loaded
/zarf-bin/busybox echo "$SHA256_ZARF  zarf" | /zarf-bin/busybox sha256sum -c
/zarf-bin/busybox echo "$SHA256_IMAGES  seed-images.tar" | /zarf-bin/busybox sha256sum -c

# Load the seed registry
/payload/zarf init bootstrap /payload/seed-images.tar library/registry:2.7.1 -l=trace
