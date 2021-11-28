#!/bin/sh

echo -e '\033[0;31m'

systemctl stop zarf-registry
systemctl disable zarf-registry
rm -f /usr/local/bin/registry
rm -f /etc/systemd/system/zarf-registry.service
rm -fr /etc/zarf-registry
rm -fr /opt/zarf-registry
systemctl daemon-reload

echo -e '\033[0m'
