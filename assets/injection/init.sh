#!/zarf-bin/busybox sh
set -ex

# Before running anything, verify the busybox binary 
/zarf-bin/busybox sha256sum -c /zarf-bin/verify-busybox.sha256

# Wait to receive files piped via tar/netcat
/zarf-bin/busybox netcat -l -p 25000 -w 360 | /zarf-bin/busybox tar xv

# Verify that the zarf assets are properly loaded
/zarf-bin/busybox sha256sum -c /zarf-bin/verify-payload.sha256

# Load the seed registry
/payload/zarf init bootstrap /payload/seed-images.tar library/registry:2.7.1 -l=trace


