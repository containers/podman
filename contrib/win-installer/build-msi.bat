@if "%1" == "" (
    @echo "usage: build-msi.bat <version>"
    @exit /b 1
)

heat dir docs -var var.ManSource -cg ManFiles -dr INSTALLDIR -gg -g1 -srd -out pages.wxs || exit /b 1
candle -ext WixUIExtension -ext WixUtilExtension -ext .\artifacts\PanelSwWixExtension.dll -arch x64 -dManSource="docs" -dVERSION="%1" podman.wxs pages.wxs podman-ui.wxs welcome-install-dlg.wxs || exit /b 1
light -ext WixUIExtension -ext WixUtilExtension -ext .\artifacts\PanelSwWixExtension.dll .\podman.wixobj .\pages.wixobj .\podman-ui.wixobj .\welcome-install-dlg.wixobj -out podman.msi || exit /b 1
