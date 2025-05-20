#!/bin/sh

echo -e '\033[0;31m'

for bin in /var/lib/rancher/k3s/data/**/bin/; do
	[ -d $bin ] && export PATH=$PATH:$bin:$bin/aux
done

set -x

for service in /etc/systemd/system/k3s*.service; do
	[ -s $service ] && systemctl stop $(basename $service)
done

for service in /etc/init.d/k3s*; do
	[ -x $service ] && $service stop
done

pschildren() {
	ps -e -o ppid= -o pid= | \
	sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
	grep -w "^$1" | \
	cut -f2
}

pstree() {
	for pid in $@; do
		echo $pid
		for child in $(pschildren $pid); do
			pstree $child
		done
	done
}

killtree() {
	kill -9 $(
		{ set +x; } 2>/dev/null;
		pstree $@;
		set -x;
	) 2>/dev/null
}

getshims() {
	ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w 'k3s/data/[^/]*/bin/containerd-shim' | cut -f1
}

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount_and_remove() {
	awk -v path="$1" '$2 ~ ("^" path) { print $2 }' /proc/self/mounts | sort -r | xargs -r -t -n 1 sh -c 'umount "$0" && rm -rf "$0"'
}

do_unmount_and_remove '/run/k3s'
do_unmount_and_remove '/var/lib/rancher/k3s'
do_unmount_and_remove '/var/lib/kubelet/pods'
do_unmount_and_remove '/var/lib/kubelet/plugins'
do_unmount_and_remove '/run/netns/cni-'

# Remove CNI namespaces
ip netns show 2>/dev/null | grep cni- | xargs -r -t -n 1 ip netns delete

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
	iface=${iface%%@*}
	[ -z "$iface" ] || ip link delete $iface
done
ip link delete cni0
ip link delete flannel.1
rm -rf /var/lib/cni/
iptables-save | grep -v KUBE- | grep -v CNI- | iptables-restore

if command -v systemctl; then
	systemctl disable k3s
	systemctl reset-failed k3s
	systemctl daemon-reload
fi

rm -f /etc/systemd/system/k3s.service

for cmd in kubectl crictl ctr; do
	if [ -L /usr/sbin/$cmd ]; then
		rm -f /usr/sbin/$cmd
	fi
done

rm -rf /etc/rancher/k3s
rm -rf /run/k3s
rm -rf /run/flannel
rm -rf /var/lib/rancher/k3s
rm -rf /var/lib/kubelet
rm -f /usr/sbin/k3s
rm -f /usr/sbin/ctr
rm -f /usr/sbin/crictl
rm -f /usr/sbin/kubectl
rm -f /opt/zarf/k3s-remove.sh
rm -fr zarf-pki

echo -e '\033[0m'
