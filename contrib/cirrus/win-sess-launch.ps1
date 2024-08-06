# Runs a script and established interactive session (session 1) and
# tunnels the output such that WSL operations will complete
$ErrorActionPreference = 'Stop'

$syms =  [char[]]([char]'a'..[char]'z') +
        [char[]]([char]'A'..[char]'Z') +
        [char[]]([char]'0'..[char]'9')

if ($Args.Length -lt 1) {
    Write-Object "Usage: " + $MyInvocation.MyCommand.Name + " <script>"
    Exit 1;
}


# Shuffle a 32 character password from $syms
function GenerateRandomPassword {
    param(
        [int]$length = 32
    )

    $rnd = [byte[]]::new($length)
    [System.Security.Cryptography.RandomNumberGenerator]::create().getBytes($rnd)
    $password = ($rnd | % { $syms[$_ % $syms.length] }) -join ''

    return $password
}

function RegenPassword {
    param($username)

    $maxAttempts = 3
    for ($attempts = 0; $attempts -lt $maxAttempts; $attempts++) {
        $password = GenerateRandomPassword

        try{
            $encPass = ConvertTo-SecureString $password -AsPlainText -Force
            Set-LocalUser -Name $username -Password $encPass
            return $password
        } catch {
            # In case we catch a flake like here
            # https://github.com/containers/podman/issues/23468
            # we want to know what happened and retry.
            Write-Host "Error generating password."
            Write-Host "Username: $username"
            Write-Host "Generated Password: $password"
            Write-Host "Error Message: $_"
        }
    }
}

$runScript = $Args[0]
$nil > tmpout
$cwd = Get-Location
Write-Output "Location: $cwd"

# Reset the password to a new random pass since it's needed in the
# clear to reauth.
$pass = RegenPassword "Administrator"

$ljob = Start-Job -ArgumentList $cwd -ScriptBlock {
    param($cwd)
    Get-Content -Wait "$cwd\tmpout"
}
$pjob = Start-Job -ArgumentList $cwd,$runScript,$pass -ScriptBlock {
    param($cwd, $runScript, $pass)
    $pwargs = @("-NonInteractive", "-WindowStyle", "hidden")
    $command = "& { powershell.exe $pwargs -File " +
               $runScript + " 3>&1 2>&1 > `"$cwd\tmpout`";" +
               "Exit `$LastExitCode }"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
    & psexec -accepteula -w $cwd -i 1 -u Administrator -p $pass `
      powershell.exe $pwargs -EncodedCommand $encoded
    if ($LASTEXITCODE -ne 0) {
       throw "failure running psexec"
    }
}

while ($pjob.State -eq 'Running') {
   Start-Sleep -Milliseconds 200
   Receive-Job $ljob
}

Start-Sleep  2
Stop-Job $ljob

while ($ljob.HasMoreData) {
  Receive-Job $ljob
  Start-Sleep -Milliseconds 200
}

if ($pjob.State -eq 'Failed') {
  Write-Output "Failure occurred, see above. Extra info:"
  Receive-Job $pjob
  throw "wsl task failed on us!"
}

Remove-Job $ljob
Remove-Job $pjob
