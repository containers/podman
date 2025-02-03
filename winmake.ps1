
#!/usr/bin/env powershell

. ./contrib/cirrus/win-lib.ps1

# Targets
function Podman-Remote{
    New-Item -ItemType Directory -Force -Path "./bin/windows"

    $buildInfo = Get-Date -UFormat %s -Millisecond 0
    $buildInfo = "-X github.com/containers/podman/v5/libpod/define.buildInfo=$buildInfo "
    $commit = Git-Commit
    $commit = "-X github.com/containers/podman/v5/libpod/define.gitCommit=$commit "

    Run-Command "go build --ldflags `"$commit $buildInfo `" --tags `"$remotetags`" --o ./bin/windows/podman.exe ./cmd/podman/."
}

function Make-Clean{
     Remove-Item ./bin -Recurse -Force -Confirm:$false
}

function Local-Machine {
    param (
    [string]$files
    );
    Build-Ginkgo
    if ($files) {
         $files = " --focus-file $files "
    }

    Run-Command "./test/tools/build/ginkgo.exe -vv  --tags `"$remotetags`" -timeout=90m --trace --no-color $files pkg/machine/e2e/."
}

# Expect starting directory to be /podman
function Win-SSHProxy {
    param (
        [string]$Version
    );

    New-Item -ItemType Directory -Force -Path "./bin/windows"
    if (-Not $Version) {
        $Version = "v0.8.3"
    }
    curl.exe -sSL -o "./bin/windows/gvproxy.exe" --retry 5 "https://github.com/containers/gvisor-tap-vsock/releases/download/$Version/gvproxy-windowsgui.exe"
    curl.exe -sSL -o "./bin/windows/win-sshproxy.exe" --retry 5 "https://github.com/containers/gvisor-tap-vsock/releases/download/$Version/win-sshproxy.exe"
}

function Installer{
    param (
        [string]$version,
        [string]$suffix = "dev"
    );
    Write-Host "Building the windows installer"

    # Check for the files to include in the installer
    $requiredArtifacts = @(
        "$PSScriptRoot\bin\windows\podman.exe"
        "$PSScriptRoot\bin\windows\gvproxy.exe"
        "$PSScriptRoot\bin\windows\win-sshproxy.exe"
        "$PSScriptRoot\docs\build\remote\podman-for-windows.html"
    )
    $requiredArtifacts | ForEach-Object {
        if (!(Test-Path -Path $PSItem -PathType Leaf)) {
            Write-Host "$PSItem not found."
            Write-Host "Make 'podman', 'win-gvproxy' and 'docs' before making the installer:"
            Write-Host "   .\winmake.ps1 podman-remote"
            Write-Host "   .\winmake.ps1 win-gvproxy"
            Write-Host "   .\winmake.ps1 docs"
            Exit 1
        }
    }

    # Create the ZIP file with the full client distribution
    $zipFileDest = "$PSScriptRoot\contrib\win-installer\current"
    Build-Distribution-Zip-File -destinationPath $zipFileDest

    if (-Not $version) {
        # Get Podman version from local source code
        $version = Get-Podman-Version
    }

    # Run \contrib\win-installer\build.ps1
    Push-Location $PSScriptRoot\contrib\win-installer
    Run-Command ".\build.ps1 $version $suffix `"$zipFileDest`""
    Pop-Location
}

function Test-Installer{
    param (
        [string]$version,
        [ValidateSet("dev", "prod")]
        [string]$flavor = "dev",
        [ValidateSet("wsl", "hyperv")]
        [string]$provider = "wsl"
    );

    if (-Not $version) {
        # Get Podman version from local source code
        $version = Get-Podman-Version
    }

    if ($flavor -eq "prod") {
        $suffix = ""
    } else {
        $suffix = "-dev"
    }

    $setupExePath = "$PSScriptRoot\contrib\win-installer\podman-${version}${suffix}-setup.exe"
    if (!(Test-Path -Path $setupExePath -PathType Leaf)) {
        Write-Host "Setup executable not found in path $setupExePath."
        Write-Host "Make 'installer' before making the installer test:"
        Write-Host "   .\winmake.ps1 installer"
        Exit 1
    }

    $command = "$PSScriptRoot\contrib\win-installer\test-installer.ps1"
    $command += " -scenario all"
    $command += " -provider $provider"
    $command += " -setupExePath $setupExePath"
    Run-Command "${command}"
}

function Documentation{
    Write-Host "Generating the documentation artifacts"
    # Check that pandoc is installed
    if (!(Get-Command -Name "pandoc" -ErrorAction SilentlyContinue)) {
        Write-Host "Pandoc not found. Pandoc is required to convert the documentation Markdown files into HTML files."
        Exit 1
    }
    # Check that the podman client is built
    $podmanClient = "$PSScriptRoot\bin\windows\podman.exe"
    if (!(Test-Path -Path $podmanClient -PathType Leaf)) {
        Write-Host "$podmanClient not found. Make 'podman-remote' before 'documentation'."
        Exit 1
    }
    Run-Command "$PSScriptRoot\docs\make.ps1 $podmanClient"
}

function Validate{
    $podmanExecutable = "podman"
    $podmanSrcVolumeMount = "${PSScriptRoot}:/go/src/github.com/containers/podman"
    # All files bind mounted from a Windows host are marked as executable.
    # That makes the pre-commit hook "check-executables-have-shebangs" fail.
    # Setting the environment variable "SKIP=check-executables-have-shebangs"
    # allow to skip that pre-commit hook.
    $podmanEnvVariable = "-e SKIP=check-executables-have-shebangs"
    $podmanRunArgs = "--rm -v $podmanSrcVolumeMount --security-opt label=disable -t -w /go/src/github.com/containers/podman $podmanEnvVariable"
    $validateImage = "quay.io/libpod/validatepr:latest"
    $validateCommand = "make .validatepr"

    # Check that podman is installed
    if (!(Get-Command -Name $podmanExecutable -ErrorAction SilentlyContinue)) {
        Write-Host "$podmanExecutable not found. $podmanExecutable is required to run the validate script."
        Exit 1
    }

    # Check that a podman machine exist
    $currentMachine = (podman machine info -f json | ConvertFrom-Json).Host.CurrentMachine
    if (!$currentMachine) {
        Write-Host "Podman machine doesn't exist. Initialize and start one before running the validate script."
        Exit 1
    }

    # Check that the podman machine is running
    $state = (podman machine info -f json | ConvertFrom-Json).Host.MachineState
    if ($state -ne "Running") {
        Write-Host "Podman machine is not running. Start the machine before running the validate script."
        Exit 1
    }

    Run-Command "$podmanExecutable run $podmanRunArgs $validateImage $validateCommand"
}

function Lint{
    # Check that golangci-lint is installed
    if (!(Get-Command -Name "golangci-lint" -ErrorAction SilentlyContinue)) {
        Write-Host "The tool ""golangci-lint"" not found. Install https://golangci-lint.run/ before running the lint script."
        Exit 1
    }

    # Check that pre-commit is installed
    if (!(Get-Command -Name "pre-commit" -ErrorAction SilentlyContinue)) {
        Write-Host "The tool ""pre-commit"" not found. Install https://pre-commit.com/ before running the lint script."
        Exit 1
    }

    Run-Command "pre-commit run --all-files"
    Run-Command "golangci-lint run --timeout=10m --build-tags=`"$remotetags`" $PSScriptRoot\cmd\podman"
}

# Helpers
function Build-Ginkgo{
    if (Test-Path -Path ./test/tools/build/ginkgo.exe -PathType Leaf) {
        return
    }
    Write-Host "Building Ginkgo"
    Push-Location ./test/tools
    Run-Command "go build -o build/ginkgo.exe ./vendor/github.com/onsi/ginkgo/v2/ginkgo"
    Pop-Location
}

function Git-Commit{
    # git is not installed by default on windows,
    # so if we can't get the commit, we don't include this info
    Get-Command git  -ErrorAction SilentlyContinue | out-null
    if(!$?){
        return
    }
    $commit = git rev-parse HEAD
    $dirty = git status --porcelain --untracked-files=no
    if ($dirty){
        $commit = "$commit-dirty"
    }
    return $commit
}

function Build-Distribution-Zip-File{
    param (
        [string]$destinationPath
        );
    $binariesFolder = "$PSScriptRoot\bin\windows"
    $documentationFolder = "$PSScriptRoot\docs\build\remote\"
    $zipFile = "$destinationPath\podman-remote-release-windows_amd64.zip"

    # Create a temporary folder to store the distribution files
    $tempFolder = New-Item -ItemType Directory -Force -Path "$env:TEMP\podman-windows"

    # Copy bin\windows\ content to the temporary folder
    Copy-Item -Recurse -Force -Path "$binariesFolder\*" -Destination "$tempFolder\"

    # Copy docs\build\remote to the temporary folder
    Copy-Item -Recurse -Force -Path "$documentationFolder" -Destination "$tempFolder\docs\"

    # If $destination path doesn't exist, create it
    if (-Not (Test-Path -Path $destinationPath -PathType Container)) {
        New-Item -ItemType Directory -Force -Path $destinationPath
    }

    # Create the ZIP file with the full client distribution
    Compress-Archive -Path "$tempFolder\*" -DestinationPath $zipFile -Force

    # Delete the temporary folder
    Remove-Item -Recurse -Force -Path "$tempFolder"
}

function Get-Podman-Version{
    $versionSrc = "$PSScriptRoot\test\version\"
    $versionBin = "$PSScriptRoot\test\version\version.exe"
    Run-Command "go build --o `"$versionBin`" `"$versionSrc`""
    $version = Invoke-Expression "$versionBin"
    # Remove the '-dev' suffix from the version
    $version = $version -replace "-.*", ""
    return $version
}

# Init script
$target = $args[0]

$remotetags = "remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp"

switch ($target) {
    {$_ -in '', 'podman-remote', 'podman'} {
        Podman-Remote
    }
    'localmachine' {
        if ($args.Count -gt 1) {
            $files = $args[1]
        }
        Local-Machine  -files $files
    }
    'clean' {
        Make-Clean
    }
    {$_ -in 'win-sshproxy', 'win-gvproxy'} {
        if ($args.Count -gt 1) {
            $ref = $args[1]
        }
        Win-SSHProxy($ref)
    }
    'installer' {
        if ($args.Count -gt 1) {
            Installer -version $args[1]
        } else {
            Installer
        }
    }
    'installertest' {
        if ($args.Count -gt 1) {
            Test-Installer -provider $args[1]
        } else {
            Test-Installer
        }
    }
    'docs' {
        Documentation
    }
    'validatepr' {
        Validate
    }
    'lint' {
        Lint
    }
    default {
        Write-Host "Usage: " $MyInvocation.MyCommand.Name "<target> [options]"
        Write-Host
        Write-Host "Example: Build podman-remote "
        Write-Host " .\winmake podman-remote"
        Write-Host
        Write-Host "Example: Run all machine tests "
        Write-Host " .\winmake localmachine"
        Write-Host
        Write-Host "Example: Run specfic machine tests "
        Write-Host " .\winmake localmachine "basic_test.go""
        Write-Host
        Write-Host "Example: Download win-gvproxy and win-sshproxy helpers"
        Write-Host " .\winmake win-gvproxy"
        Write-Host
        Write-Host "Example: Build the windows installer"
        Write-Host " .\winmake installer"
        Write-Host
        Write-Host "Example: Run windows installer tests"
        Write-Host " .\winmake installertest hyperv"
        Write-Host
        Write-Host "Example: Generate the documetation artifacts"
        Write-Host " .\winmake docs"
        Write-Host
        Write-Host "Example: Validate code changes before submitting a PR"
        Write-Host " .\winmake validatepr"
        Write-Host
        Write-Host "Example: Run linters"
        Write-Host " .\winmake lint"
    }
}
