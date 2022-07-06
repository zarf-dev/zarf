---
sidebar_position: 8
---
# Supported OSes

Zarf is intended to install & run on a multitude of 64-bit Linux distributions.

Check the table below to understand which distros which we test against & if there are any known issues / usage caveats.

&nbsp;

<!-- TODO: @JPERRY this support matrix could probably just go into a FAQ?  -->
## Support Matrix

|OS             |VM_ID      |Notes|
|---            |---        |---|
|RHEL 7         |rhel7      ||
|RHEL 8         |rhel8      ||
|CentOS 7       |centos7    ||
|CentOS 8       |centos8    ||
|Ubuntu 20.04   |ubuntu     ||
|Debian 11      |debian     ||
|Rocky 8.4      |rocky      ||

&nbsp;

<!-- TODO: @JPERRY Is any of the content below this comment actually useful? -->
## Demo Environments

We support running an instance of Zarf _inside a local VM_ (of any of the [supported OSes](#support-matrix)) for test & demonstration purposes.

> _**Take note**_
>
> Run the following commands from  _**the project root directory**_.

&nbsp;

### Startup

To get a VM running, it's as easy as running a single command:

```sh
make vm-init OS=[VM_ID]     # e.g. make vm-init OS=ubuntu
```

> _**Warning!**_
>
> Besure to pass a VM_ID or you'll start a VM instance for _every one of the supported OS types_. Yikes!

&nbsp;


### Work in the VM

To connect into the VM instance you just started, run:

```sh
vagrant ssh [VM_ID]         # e.g. vagrant ssh ubuntu
```

Once connected, you can work with your mounted-from-the-host copy of Zarf like so:

```sh
sudo su                     # escalate permissions (to "root" user)
cd /opt/zarf                # access Zarf
./zarf help
```

When you're done with the VM, you can exit back to the host terminal by running:

```sh
exit                        # de-escalate permissions (back to "vagrant" user)
exit                        # exits VM shel & drops you back on the host
```

&nbsp;


### Shutdown

Closing out the demo environment is _also_ a single command:

```sh
make vm-destroy
```

This will shutdown & destroy _all_ the demo VM instances it can find.  Easy-peasy&mdash;nice and clean.