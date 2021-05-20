Vagrant.configure("2") do |config|

  config.vm.define "rhel7" do |rhel7|
    rhel7.vm.box = "generic/rhel7"
    rhel7.vm.hostname = "rhel"
  end
  
  config.vm.define "rhel8" do |rhel8|
    rhel8.vm.box = "generic/rhel8"
    rhel8.vm.hostname = "rhel"
  end
  
  config.vm.define "ubuntu" do |ubuntu|
    ubuntu.vm.box = "ubuntu/focal64"
    ubuntu.vm.hostname = "ubuntu"
  end

  config.vm.synced_folder '.', '/vagrant', disabled: true
  config.vm.synced_folder 'build', '/opt/shift', SharedFoldersEnableSymlinksCreate: false
  
  config.vm.network "forwarded_port", guest: 80, host: 8080

  config.vm.disk :disk, size: "20GB", primary: true
  config.ssh.insert_key = false
  
  config.vm.provider "virtualbox" do |vb|
    vb.check_guest_additions = false
    vb.cpus = 4
    vb.memory = 4096
  end

  config.vm.provision "shell", inline: <<-SHELL
    cd /opt/shift
    # Airgap images please
    echo "0.0.0.0 registry.dso.mil registry1.dso.mil index.docker.io auth.docker.io registry-1.docker.io dseasb33srnrn.cloudfront.net production.cloudflare.docker.com" >> /etc/hosts
    ./shift-pack initialize --dryrun=false
  SHELL

end
