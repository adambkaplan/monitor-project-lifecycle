# Contributing

## Development Setup

* Fedora: Install packages `dnf install -y golang git gcc rpm-build make docker createrepo`
* CentOS/RHEL: Install packages `yum install -y golang git gcc rpm-build make docker createrepo`
* MacOS: use [Homebrew](https://brew.sh/) to: 
    * Install packages `brew install golang git`
    * Install the following:
        * [Docker CE](https://store.docker.com/editions/community/docker-ce-desktop-mac)
        * [VirtualBox](https://www.virtualbox.org/wiki/Downloads)
        * [Vagrant](https://www.vagrantup.com/downloads.html)

      OR

    * Install casks `brew cask install docker virtualbox vagrant`. Note - most casks require `sudo` permissions to complete installation, and require additional actions to be taken. Follow post-installation instructions carefully.
    * Once Vagrant is installed, install the Vagrant guest plugins - `vagrant plugin install vagrant-vbguest`

Once all dependencies are installed, ensure that you have a properly configured [Go workspace](https://golang.org/doc/code.html#Workspaces) on your machine.

## Building

### Linux

To build the binary, run
```
$ make
```
To build the RPM and images, run
```
$ OS_BUILD_ENV_PRESERVE=_output/local/bin hack/env make build-images
```

### MacOS

To build the binary, run
```
$ make
```
To build the RPM and images, launch and ssh into a virtual machine via Vagrant:
```
$ vagrant up && vagrant ssh
```
Next, navigate to the go workspace within the VM and run the `build-images` command as above for [Linux](#linux)
```
$ cd /go/src/github.com/adambkaplan/openshift-template-monitor
$ OS_BUILD_ENV_PRESERVE=_output/local/bin hack/env make build-images
```

## Updating Go Dependencies

Dependencies are managed by [glide](https://glide.sh/).
To avoid circular dependency conflicts, use the `--strip-vendor` option when executing all `glide` commands.
