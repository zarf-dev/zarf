Vagrant.configure("2") do |config|

  config.vm.provider "virtualbox" do |vb|
      vb.check_guest_additions = false
      vb.cpus = 6
      vb.memory = 8192
    end

  config.vm.disk :disk, size: "20GB", primary: true

  config.vm.define "rhel7" do |target|
    target.vm.box = "generic/rhel7"
  end

  config.vm.define "rhel8" do |target|
    target.vm.box = "generic/rhel8"
  end

  config.vm.define "centos7" do |target|
    target.vm.box = "boxomatic/centos-7"
  end

  config.vm.define "centos8" do |target|
    target.vm.box = "boxomatic/centos-8"
  end

  config.vm.define "ubuntu" do |target|
      target.vm.box = "boxomatic/ubuntu-20.04"
    end

  config.vm.define "bigbangdemo" do |target|
    # Install vagrant-disksize to allow resizing the vagrant box disk.
    unless Vagrant.has_plugin?("vagrant-disksize")
        raise  Vagrant::Errors::VagrantError.new, "vagrant-disksize plugin is missing. Please install it using 'vagrant plugin install vagrant-disksize' and rerun 'vagrant up'"
    end
    config.vm.provider "virtualbox" do |vb|
      vb.check_guest_additions = false
      vb.cpus = 8
      vb.memory = 28672
    end
    config.disksize.size = "100GB"
    config.vm.disk :disk, size: "100GB", primary: true
    target.vm.box = "boxomatic/ubuntu-20.04"
    config.vm.provision "shell", inline: <<-SHELL
      # The partition is 100GB but the filesystem isn't yet
      growpart /dev/sda 1 && resize2fs /dev/sda1

      # Elasticsearch needs this
      sysctl -w vm.max_map_count=262144
      SHELL
  end

  config.vm.define "debian" do |target|
    target.vm.box = "boxomatic/debian-11"
  end

  config.vm.define "rocky" do |target|
    target.vm.box = "boxomatic/rocky-8.4"
  end

  config.vm.hostname = "zarf-test"
  config.vm.synced_folder '.', '/vagrant', disabled: true
  config.vm.synced_folder 'build', '/opt/zarf', SharedFoldersEnableSymlinksCreate: false

  config.vm.network "forwarded_port", guest: 80, host: 80
  config.vm.network "forwarded_port", guest: 443, host: 443
  config.vm.network "forwarded_port", guest: 8080, host: 8080
  config.vm.network "forwarded_port", guest: 8443, host: 8443

  config.ssh.insert_key = false

  config.vm.provision "shell", inline: <<-SHELL
    # Airgap images please
    echo "0.0.0.0 registry.hub.docker.com hub.docker.com charts.helm.sh repo1.dso.mil github.com registry.dso.mil registry1.dso.mil docker.io index.docker.io auth.docker.io registry-1.docker.io dseasb33srnrn.cloudfront.net production.cloudflare.docker.com" >> /etc/hosts
    SHELL
end
