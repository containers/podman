@if "%1" == "" (
    @echo "usage: build-burn.bat <version>"
    @exit /b 1
)

candle -ext WixUIExtension -ext WixUtilExtension -ext WixBalExtension -arch x64 -dManSource="docs" -dVERSION="%1" burn.wxs || exit /b 1
light -ext WixUIExtension -ext WixUtilExtension -ext WixBalExtension .\burn.wixobj -out podman-setup.exe || exit /b 1
