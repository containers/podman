![PODMAN logo](../../logo/podman-logo-source.svg)

# Podman in VS Code

This tutorial shows you how to set up **development containers** inside VS Code to achieve reproducible builds. In this example we will use a `devcontainer.json` file that also makes your project compatible with GitHub Codespaces and GitHub Actions. However, these instructions can also work for other project hosting and continuous integration services.

In this example we will be using the common scenario of using GitHub Pages to host your blog using Jekyll.

## Installing Podman and VS Code

For installing or building Podman, please see the [installation instructions](https://podman.io/getting-started/installation).

## Set up your blog source code

Create a new directory on your Desktop and add a file inside named `index.md` with these contents:

```md
# My blog

Here is my blog about horses.
```

## Add devcontainer

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
	"remoteUser": "vscode"
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

## Set up VS Code

Install VS Code and add the Containers Remote Development extension in the extensions marketplace.

NOTE: :warning: :warning::warning::warning:warning::warning:  :    THIS PART OF THE INSTRUCTIONS NEEDS HELP TO MAKE PODMAN WORK PROPERLY

## Run the container in VS Code

Use VS Code to open your project folder on your desktop.

VS Code should ask you if you want to start up a development container for this project.

> Folder contains a Dev Container configuration file. Reopen folder to develop in a container (learn more).

Click YES and wait for the container to build.

Use COMMAND-~ to open the terminal (which is inside the container) and run this command to serve your blog using Jekyll:

```sh
bundle exec jekyll serve
```

## Open the blog in your browser

From the previous step, you should see a link for localhost:4000/, click that link.

Your browser will now open to display the page!

Reviewing what just happened, Jekyll is running inside your container. You are connected to that container using the terminal in VS Code. VS Code has detected/forwarded the required open ports. And, if your ports were forwarded using a different port, VS Code would also have noticed the URL in the terminal command output and rewritten that link for you. All this happened without having to install Ruby or Jekyll on your local logical machine.

## Further reading

An example of a more complicated blog using this technique is at https://github.com/fulldecent/blog.phor.net/.

See https://github.com/features/codespaces. If your repository is checked into GitHub then you are able to perform all these using the web interface in GitHub on the repository page under the CODE button then the CODESPACES tab. Charges may apply.

See https://github.com/features/actions. If your repository is hosted on GitHub you can also use the work above as a starting point for building your continuous integration/continuous deployment workflows.

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands)
page.
