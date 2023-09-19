# Targets
function Podman-Remote{
    New-Item  ./bin/windows -ItemType Directory -ea 0

    $buildInfo = Get-Date -UFormat %s -Millisecond 0
    $buildInfo = "-X github.com/containers/podman/v4/libpod/define.buildInfo=$buildInfo "
    $commit = Git-Commit
    $commit = "-X github.com/containers/podman/v4/libpod/define.gitCommit=$commit "

    Write-Host go build --ldflags "$commit $buildInfo " --tags "$remotetags" --o .\bin\windows\podman.exe ./cmd/podman/.
    go build --ldflags "$commit $buildInfo " --tags "$remotetags" --o .\bin\windows\podman.exe ./cmd/podman/.
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

    ./test/tools/build/ginkgo.exe -vv  --tags "$remotetags" -timeout=90m --trace --no-color $files pkg/machine/e2e/.
}

# Helpers
function Build-Ginkgo{
    if (Test-Path -Path ./test/tools/build/ginkgo.exe -PathType Leaf) {
        return
    }
    Push-Location ./test/tools
    go build -o build/ginkgo.exe ./vendor/github.com/onsi/ginkgo/v2/ginkgo
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
