$ErrorActionPreference = 'Stop'

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
        [string]$Ref
    );

    # git is not installed by default on windows
    # fail if git doesn't exist
    Get-Command git -ErrorAction SilentlyContinue  | out-null
    if(!$?){
        Write-Host "Git not installed, cannot build Win-SSHProxy"
        Exit 1
    }

    if (Test-Path ./tmp-gv) {
        Remove-Item ./tmp-gv -Recurse -Force -Confirm:$false
    }

    $GV_GITURL = "https://github.com/containers/gvisor-tap-vsock.git"

    New-Item  ./tmp-gv -ItemType Directory -ea 0
    Push-Location ./tmp-gv
    Run-Command "git init"
    Run-Command "git remote add origin $GV_GITURL"
    Run-Command "git fetch --depth 1 origin main"
    Run-Command "git fetch --depth 1 --tags"
    Run-Command "git checkout main"
    if (-Not $Ref) {
        Write-Host "empty"
        $Ref = git describe --abbrev=0
    }
    Run-Command "git checkout $Ref"
    Run-Command "go build -ldflags -H=windowsgui -o bin/win-sshproxy.exe ./cmd/win-sshproxy"
    Run-Command "go build -ldflags -H=windowsgui -o bin/gvproxy.exe ./cmd/gvproxy"
    Pop-Location

    # Move location to ./bin/windows for packaging script and for Windows binary testing
    New-Item -ItemType Directory -Force -Path "./bin/windows"
    Copy-Item -Path "tmp-gv/bin/win-sshproxy.exe" -Destination "./bin/windows/"
    Copy-Item -Path "tmp-gv/bin/gvproxy.exe" -Destination "./bin/windows/"
    Remove-Item ./tmp-gv -Recurse -Force -Confirm:$false
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

function Run-Command {
    param (
        [string] $command
    )

    Write-Host $command

    Invoke-Expression $command
    $result = $LASTEXITCODE
    if ($result -ne 0) {
        Write-Host "Command failed (exit: $result)"
        Exit $result
    }
}

# Init script
$target = $args[0]

$remotetags = "remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp"
$Env:GOOS = "windows"; $Env:GOARCH = "amd64"

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
