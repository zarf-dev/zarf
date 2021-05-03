Vagrant.configure("2") do |config|

  # config.vm.define "rhel7" do |rhel7|
  #   rhel7.vm.box = "generic/rhel7"
  # end
  
  config.vm.define "ubuntu" do |ubuntu|
    ubuntu.vm.box = "ubuntu/focal64"
  end

  config.vm.synced_folder '.', '/vagrant', disabled: true
  config.vm.synced_folder 'build', '/opt/shift', SharedFoldersEnableSymlinksCreate: false
  
  config.vm.network "private_network", ip: "172.16.10.10"
  config.vm.disk :disk, size: "20GB", primary: true
  config.ssh.insert_key = false
  
  config.vm.provider "virtualbox" do |vb|
    vb.check_guest_additions = false
    vb.cpus = 4
    vb.memory = 4096
  end

  config.vm.provision "shell", inline: <<-SHELL
    cd /opt/shift
    ./shift-pack initialize
  SHELL

end
