cd ../..
go build -buildmode=c-shared  -o contrib/win-installer/artifacts/podman-msihooks.dll ./cmd/podman-msihooks || exit /b 1
go build -ldflags -H=windowsgui -o contrib/win-installer/artifacts/podman-wslkerninst.exe ./cmd/podman-wslkerninst || exit /b 1
cd contrib/win-installer
