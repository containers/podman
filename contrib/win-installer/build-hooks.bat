cd ../..
set GOARCH=amd64
go build -ldflags -H=windowsgui -o contrib/win-installer/artifacts/podman-wslkerninst.exe ./cmd/podman-wslkerninst || exit /b 1
cd contrib/win-installer
@rem Build using x86 toolchain, see comments in check.c for rationale and details
x86_64-w64-mingw32-gcc podman-msihooks/check.c -shared -lmsi -mwindows -o artifacts/podman-msihooks.dll || exit /b 1
