# Runs a script and established interactive session (session 1) and
# tunnels the output such that WSL operations will complete
$ErrorActionPreference = 'Stop'

if ($Args.Length -lt 1) {
    Write-Object "Usage: " + $MyInvocation.MyCommand.Name + " <script>"
    Exit 1;
}

function RegenPassword {
    param($username)
    $syms = [char[]]([char]'a'..[char]'z' `
               + [char]'A'..[char]'Z'  `
               + [char]'0'..[char]'9')
    $rnd = [byte[]]::new(32)
    [System.Security.Cryptography.RandomNumberGenerator]::create().getBytes($rnd)
    $password = ($rnd | % { $syms[$_ % $syms.length] }) -join ''
    $encPass = ConvertTo-SecureString $password -AsPlainText -Force
    Set-LocalUser -Name $username -Password $encPass
    return $password
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
  Write-Output "Failure occured, see above. Extra info:"
  Receive-Job $pjob
  throw "wsl task failed on us!"
}

Remove-Job $ljob
Remove-Job $pjob
