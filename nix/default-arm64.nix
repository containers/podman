let
  pkgs = (import ./nixpkgs.nix {
    crossSystem = {
      config = "aarch64-unknown-linux-gnu";
    };
    config = {
      packageOverrides = pkg: {
        gpgme = (static pkg.gpgme);
        libassuan = (static pkg.libassuan);
        libgpgerror = (static pkg.libgpgerror);
        libseccomp = (static pkg.libseccomp);
        glib = (static pkg.glib).overrideAttrs (x: {
          outputs = [ "bin" "out" "dev" ];
          mesonFlags = [
            "-Ddefault_library=static"
            "-Ddevbindir=${placeholder ''dev''}/bin"
            "-Dgtk_doc=false"
            "-Dnls=disabled"
          ];
          postInstall = ''
            moveToOutput "share/glib-2.0" "$dev"
            substituteInPlace "$dev/bin/gdbus-codegen" --replace "$out" "$dev"
            sed -i "$dev/bin/glib-gettextize" -e "s|^gettext_dir=.*|gettext_dir=$dev/share/glib-2.0/gettext|"
            sed '1i#line 1 "${x.pname}-${x.version}/include/glib-2.0/gobject/gobjectnotifyqueue.c"' \
              -i "$dev"/include/glib-2.0/gobject/gobjectnotifyqueue.c
          '';
        });
        pcsclite = (static pkg.pcsclite).overrideAttrs (x: {
          configureFlags = [
            "--enable-confdir=/etc"
            "--enable-usbdropdir=/var/lib/pcsc/drivers"
            "--disable-libsystemd"
            "--disable-libudev"
            "--disable-libusb"
          ];
          buildInputs = [ pkgs.python3 pkgs.dbus ];
        });
        systemd = (static pkg.systemd).overrideAttrs (x: {
          outputs = [ "out" "dev" ];
          mesonFlags = x.mesonFlags ++ [
            "-Dglib=false"
            "-Dstatic-libsystemd=true"
          ];
        });
      };
    };
  });

  static = pkg: pkg.overrideAttrs (x: {
    doCheck = false;
    configureFlags = (x.configureFlags or [ ]) ++ [
      "--without-shared"
      "--disable-shared"
    ];
    dontDisableStatic = true;
    enableSharedExecutables = false;
    enableStatic = true;
  });

  self = with pkgs; buildGoModule rec {
    name = "podman";
    src = builtins.filterSource
      (path: type: !(type == "directory" && baseNameOf path == "bin")) ./..;
    vendorSha256 = null;
    doCheck = false;
    enableParallelBuilding = true;
    outputs = [ "out" ];
    nativeBuildInputs = [ bash gitMinimal go-md2man pkg-config which ];
    buildInputs = [ glibc glibc.static glib gpgme libassuan libgpgerror libseccomp libapparmor libselinux ];
    prePatch = ''
      export CFLAGS='-static -pthread'
      export LDFLAGS='-s -w -static-libgcc -static'
      export EXTRA_LDFLAGS='-s -w -linkmode external -extldflags "-static -lm"'
      export BUILDTAGS='static netgo osusergo exclude_graphdriver_btrfs exclude_graphdriver_devicemapper seccomp apparmor selinux'
      export CGO_ENABLED=1
    '';
    buildPhase = ''
      patchShebangs .
      make bin/podman
      make bin/podman-remote
      make bin/rootlessport
    '';
    installPhase = ''
      install -Dm755 bin/podman $out/bin/podman
      install -Dm755 bin/podman-remote $out/bin/podman-remote
      install -Dm755 bin/rootlessport $out/libexec/podman/rootlessport
    '';
  };
in
self
