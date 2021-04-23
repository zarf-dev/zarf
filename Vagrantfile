Vagrant.configure("2") do |config|
  #  config.vm.box = "generic/rhel7"
  config.vm.box = "ubuntu/focal64"
  
  # config.vm.synced_folder '.', '/vagrant', disabled: true
  
  config.vm.network "private_network", ip: "172.16.10.10"
  config.vm.disk :disk, size: "100GB", primary: true

  config.vm.provider "virtualbox" do |vb|
    vb.check_guest_additions = false
    vb.cpus = 6
    vb.memory = 8192
  end

  # config.vm.provision "shell", path: "bin/shift-package", privileged: true
end
