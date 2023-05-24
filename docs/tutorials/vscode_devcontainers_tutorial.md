![PODMAN logo](../../logo/podman-logo-source.svg)

# DevContainers in Visual Studio Code with Podman

This tutorial shows you how to set up **development containers** inside VS Code to achieve reproducible builds. In this example we will use a `devcontainer.json` file that also makes your project compatible with GitHub Codespaces and GitHub Actions. However, these instructions can also work for other project hosting and continuous integration services.

In this example we will be using the common scenario of using GitHub Pages to host your blog using Jekyll.

## Installing Podman and VS Code

For installing or building Podman, please see the [installation instructions](https://podman.io/getting-started/installation).

**IMPORTANT**: When installing Visual Studio Code, it is important that you are not using the Flatpak version of the software, as it has limitations that will not allow you to successfully set up development containers.

## Set up Visual Studio Code 

The first step after making sure Podman and Visual Studio Code are installed is to set up your environment. 

### Install Extensions

Install the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) and [Docker](https://marketplace.visualstudio.com/items?itemName=ms-azuretools.vscode-docker) extensions in Visual Studio Code.

### Configure Extensions
- Go to the Dev Containers extension settings
- Change the Docker Executable path to "podman"
- Change the Docker Compose path to "podman-compose"
	- If you do not have podman-compose installed, please see the [installation instructions](https://github.com/containers/podman-compose)
<br/><br/>
- Go to the Docker extension settings
- Change the Docker Path to the absolute path to the Podman binary
- Add the environment variable DOCKER_HOST with the value that points to the podman socket:
    - If you are running a Linux machine, you would give it the value: unix:///run/user/1000/podman/podman.sock
    - If you are running a MacOS machine, you would give it the value: unix:///Users/$USER/.local/share/podman/podman-machine-default/podman.sock
    - If you are running a Windows machine, you would give it the value: <Needs to be Added>

### Set up your first Development Container

Now that you have your environment set up and your extensions are configured properly, we can start creating development containers. 
- If you are using MacOS or Windows, make sure you set up Podman by running `podman machine init` and then `podman machine start`
- After making sure that Podman is ready to go, you need to create and/or open the project where you want to set up the development container. 
- Using the Command Palette in Visual Studio Code, begin typing and select the option "Dev Containers: Add Dev Container Configuration Files..."
- Select your base image of choice
- Select any additional features to install inside the container and press "OK"
	- This will create a `.devcontainer` directory in your workspace that contains a `devcontainer.json` file. This file tells Visual Studio Code how to access or create your development container
- Next you need to make some modifications to the `devcontainer.json` file that is provided to you by the extension in order to get it working with Podman
	- If you are running a Linux machine, you need to have the extension mount your workspace with the proper SELinux context. This context is what access control is based off of. You additionally need to map your current uid and gid to the same values in the container by creating a user namespace:
	``` json
	// Add the following lines to your devcontainer.json file to mount the workspace with the proper SELinux context
	"workspaceMount": "source=${localWorkspaceFolder},target=/workspace,type=bind,Z",
	"workspaceFolder": "/workspace",

	// Add the following lines to your devcontainer.json file to map your current uid and gid to the container
	"runArgs": ["--userns=keep-id:uid=1000,gid=1000"],
	"containerUser": "vscode",
	```
    - If you are running a MacOS machine, you need to add the following line:
    ```json
    "runArgs": ["--userns=keep-id:uid=1000,gid=1000"],
    "containerUser": "vscode",
    ```
- After making those modifications, you can run your first development container! Using the Command Palette in Visual Studio Code, begin typing and select the option "Dev Containers: Rebuild and Reopen in Container"
	- This will begin setting up your development container
	- Once the extension is done setting up your container, pull up the terminal and you can interact with the container! Feel free to create files and make changes to them. 

## Example: Creating a Blog 
### Set up your blog source code

Create a new directory on your Desktop and add a file inside named `index.md` with these contents:

```md
# My blog

Here is my blog about horses.
```

### Add devcontainer

Add a folder inside your project folder named `.devcontainer`. And inside add these files:

`devcontainer.json`

```json
// For format details, see https://aka.ms/devcontainer.json. For config options, see the README at:
// https://github.com/microsoft/vscode-dev-containers/tree/v0.241.1/containers/jekyll
{
	"name": "Jekyll",
	"build": {
		"dockerfile": "Dockerfile",
		"args": {
			// Update 'VARIANT' to pick a Debian OS version: bullseye, buster
			// Use bullseye when on local arm64/Apple Silicon.
			"VARIANT": "bullseye",
			// Enable Node.js: pick the latest LTS version
			"NODE_VERSION": "lts/*"
		}	
	},

	// Use 'forwardPorts' to make a list of ports inside the container available locally.
	"forwardPorts": [
		// Jekyll server
		4000,
		// Live reload server
		35729
	],

	// Use 'postCreateCommand' to run commands after the container is created.
	"postCreateCommand": "sh .devcontainer/post-create.sh",

	// Comment out to connect as root instead. More info: https://aka.ms/vscode-remote/containers/non-root.
	"remoteUser": "vscode",

	"workspaceMount": "source=${localWorkspaceFolder},target=/workspace,type=bind,Z",
	"workspaceFolder": "/workspace",

	"runArgs": ["--userns=keep-id"],
	"containerUser": "vscode"
}
```

`Dockerfile`

```dockerfile
# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.241.1/containers/jekyll/.devcontainer/base.Dockerfile

# [Choice] Debian OS version (use bullseye on local arm64/Apple Silicon): bullseye, buster
ARG VARIANT="2.7-bullseye"
FROM mcr.microsoft.com/vscode/devcontainers/jekyll:0-${VARIANT}

# [Choice] Node.js version: none, lts/*, 16, 14, 12, 10
ARG NODE_VERSION="none"
RUN if [ "${NODE_VERSION}" != "none" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

# [Optional] Uncomment this section to install additional OS packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
```

`post-create.sh`

```sh
#!/bin/sh

# Install the version of Bundler.
if [ -f Gemfile.lock ] && grep "BUNDLED WITH" Gemfile.lock > /dev/null; then
    cat Gemfile.lock | tail -n 2 | grep -C2 "BUNDLED WITH" | tail -n 1 | xargs gem install bundler -v
fi

# If there's a Gemfile, then run `bundle install`
# It's assumed that the Gemfile will install Jekyll too
if [ -f Gemfile ]; then
    bundle install
fi
```

NOTE: perhaps these recommended files can be improved and add citation to best practices.

### Run the container in VS Code

Use VS Code to open your project folder on your desktop.

VS Code should ask you if you want to start up a development container for this project.

> Folder contains a Dev Container configuration file. Reopen folder to develop in a container (learn more).

Click YES and wait for the container to build.

Use COMMAND-~ to open the terminal (which is inside the container) and run this command to serve your blog using Jekyll:

```sh
bundle exec jekyll serve
```

### Open the blog in your browser

From the previous step, you should see a link for localhost:4000/, click that link.

Your browser will now open to display the page!

Reviewing what just happened, Jekyll is running inside your container. You are connected to that container using the terminal in VS Code. VS Code has detected/forwarded the required open ports. And, if your ports were forwarded using a different port, VS Code would also have noticed the URL in the terminal command output and rewritten that link for you. All this happened without having to install Ruby or Jekyll on your local logical machine.

## Further reading

An example of a more complicated blog using this technique is at https://github.com/fulldecent/blog.phor.net/.

See https://github.com/features/codespaces. If your repository is checked into GitHub then you are able to perform all these using the web interface in GitHub on the repository page under the CODE button then the CODESPACES tab. Charges may apply.

See https://github.com/features/actions. If your repository is hosted on GitHub you can also use the work above as a starting point for building your continuous integration/continuous deployment workflows.

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands)
page.

