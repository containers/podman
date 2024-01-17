
. ./contrib/cirrus/win-lib.ps1

# Targets
function Podman-Remote{
    New-Item -ItemType Directory -Force -Path "./bin/windows"

    $buildInfo = Get-Date -UFormat %s -Millisecond 0
    $buildInfo = "-X github.com/containers/podman/v4/libpod/define.buildInfo=$buildInfo "
    $commit = Git-Commit
    $commit = "-X github.com/containers/podman/v4/libpod/define.gitCommit=$commit "

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

    Run-Command "./test/tools/build/ginkgo.exe -vv  --tags `"$remotetags`" -timeout=90m --trace --no-color $files pkg/machine/e2e/. "
}

# Expect starting directory to be /podman
function Win-SSHProxy {
    param (
        [string]$Version
    );

    New-Item -ItemType Directory -Force -Path "./bin/windows"
    if (-Not $Version) {
        $Version = "v0.7.2"
    }
    curl.exe -sSL -o "./bin/windows/gvproxy.exe" --retry 5 "https://github.com/containers/gvisor-tap-vsock/releases/download/$Version/gvproxy-windowsgui.exe"
    curl.exe -sSL -o "./bin/windows/win-sshproxy.exe" --retry 5 "https://github.com/containers/gvisor-tap-vsock/releases/download/$Version/win-sshproxy.exe"
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
    Get-Command git  -ErrorAction SilentlyContinue  | out-null
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
    'win-sshproxy' {
        if ($args.Count -gt 1) {
            $ref = $args[1]
        }
        Win-SSHProxy -Ref $ref
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
    }
}
