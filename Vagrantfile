# -*- mode: ruby -*-
# vi: set ft=ruby :

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure(2) do |config|
    config.vm.provider "libvirt" do |libvirt, override|
        libvirt.memory = 3096
        libvirt.cpus = 3
	libvirt.storage :file,
		:type => 'qcow2'
    end
    config.vm.synced_folder ".", "/home/vagrant/sync", disabled: true
    config.vm.synced_folder ".", "/home/vagrant/libpod", type: "rsync", rsync__exclude: ["_output"]

  # The most common configuration options are documented and commented below.
  # For a complete reference, please see the online documentation at
  # https://docs.vagrantup.com.

  # Every Vagrant development environment requires a box. You can search for
  # boxes at https://atlas.hashicorp.com/search.
    config.vm.define "fedora_atomic" do |fedora_atomic|
        fedora_atomic.vm.box = "fedora_atomic"
        fedora_atomic.vm.box_url = "https://getfedora.org/atomic_vagrant_libvirt_latest"
    end
    config.vm.define "centos_atomic" do |centos_atomic|
        centos_atomic.vm.box =  "centos_atomic"
        centos_atomic.vm.box_url = "https://ci.centos.org/artifacts/sig-atomic/centos-continuous/images/cloud/latest/images/centos-atomic-host-7-vagrant-libvirt.box"
    end
    config.vm.define "fedora_cloud" do |fedora_cloud|
        fedora_cloud.vm.box =  "fedora/26-cloud-base"
    end
end
