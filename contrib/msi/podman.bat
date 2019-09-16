@echo off
setlocal enableextensions

title Podman

if "%1" EQU "" (
  goto run_help
)

if "%1" EQU "/?" (
  goto run_help
)

:: If remote-host is given on command line -- use it
setlocal enabledelayedexpansion
for %%a in (%*) do (
  echo "%%a" |find "--remote-host" >NUL
  if !errorlevel! == 0 (
    goto run_podman
  )
)

:: If PODMAN_VARLINK_BRIDGE is set -- use it
if defined PODMAN_VARLINK_BRIDGE (
  goto run_podman
)

:: If the configuration file exists -- use it
set config_home=%USERPROFILE%\AppData\podman
set config_file=%config_home%\podman-remote.conf
if exist "%config_file%" (
  goto run_podman
)

:: Get connection information from user and build configuration file
md "%config_home%"
set /p host="Please enter the remote hosts name or IP address: "
set /p user="Please enter the remote user name: "
(
  echo [connections]
  echo   [connections."%host%"]
  echo   destination = "%host%"
  echo   username = "%user%"
  echo   default = true
) >"%config_file%"

:run_podman
endlocal
podman-remote-windows.exe %*
goto end

:run_help
set run=start "Podman Help" /D "%~dp0" /B

if not "%3" == "" (
  %run% "podman-%2-%3.html
  goto end
)

if not "%2" == "" (
  %run% "podman-%2.html
  goto end
)

%run% "%podman-remote.html"
goto end

:End
