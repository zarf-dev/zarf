Vagrant.configure("2") do |config|

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

  config.vm.define "debian" do |target|
    target.vm.box = "boxomatic/debian-11"
  end

  config.vm.define "rocky" do |target|
    target.vm.box = "boxomatic/rocky-8.4"
  end

  config.vm.hostname = "zarf-test"
  config.vm.synced_folder '.', '/vagrant', disabled: true
  config.vm.synced_folder 'build', '/opt/zarf', SharedFoldersEnableSymlinksCreate: false
  
  config.vm.network "forwarded_port", guest: 80, host: 8080
  config.vm.network "forwarded_port", guest: 443, host: 8443

  config.vm.disk :disk, size: "20GB", primary: true
  config.ssh.insert_key = false
  
  config.vm.provider "virtualbox" do |vb|
    vb.check_guest_additions = false
    vb.cpus = 8
    vb.memory = 28672
  end

  config.vm.provision "shell", inline: <<-SHELL
    cd /opt/zarf
    # Airgap images please
    echo "0.0.0.0 registry.hub.docker.com hub.docker.com charts.helm.sh repo1.dso.mil github.com registry.dso.mil registry1.dso.mil docker.io index.docker.io auth.docker.io registry-1.docker.io dseasb33srnrn.cloudfront.net production.cloudflare.docker.com" >> /etc/hosts
    # ./zarf init --confirm --host=localhost
  SHELL

end
