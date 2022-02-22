#!/zarf-bin/busybox sh
set -ex

# Before /zarf-bin/busyboxning anything, verify the busybox binary 
/zarf-bin/busybox echo "$SHA256_BUSYBOX  /zarf-bin/busybox" | /zarf-bin/busybox sha256sum -c

# Wait to receive the tarball via netcat
/zarf-bin/busybox netcat -l -p 5000 -v -n -o traffic | /zarf-bin/busybox tar xv

sleep 9999

# # Extract the archive
# /zarf-bin/busybox tar xvf payload.tar

# # Verify that the zarf assets are properly loaded
# /zarf-bin/busybox echo "$SHA256_ZARF  zarf-registry" | /zarf-bin/busybox sha256sum -c
# /zarf-bin/busybox echo "$SHA256_IMAGE  seed-image.tar" | /zarf-bin/busybox sha256sum -c

# # Load the seed registry
# /payload/zarf-registry /payload/seed-image.tar $SEED_IMAGE
