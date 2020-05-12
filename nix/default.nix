let
  pkgs = import ./nixpkgs.nix {
    config = {
      packageOverrides = pkg: {
        go_1_12 = pkg.go_1_14;
      };
    };
  };

  static = pkg: pkg.overrideAttrs(old: {
    configureFlags = (old.configureFlags or []) ++
      [ "--without-shared" "--disable-shared" ];
    dontDisableStatic = true;
    enableSharedExecutables = false;
    enableStatic = true;
  });

  patchLvm2 = pkg: pkg.overrideAttrs(old: {
    configureFlags = [
      "--disable-cmdlib" "--disable-readline" "--disable-udev_rules"
      "--disable-udev_sync" "--enable-pkgconfig" "--enable-static_link"
    ];
    preConfigure = old.preConfigure + ''
      substituteInPlace libdm/Makefile.in --replace \
        SUBDIRS=dm-tools SUBDIRS=
      substituteInPlace tools/Makefile.in --replace \
        "TARGETS += lvm.static" ""
      substituteInPlace tools/Makefile.in --replace \
        "INSTALL_LVM_TARGETS += install_tools_static" ""
    '';
    postInstall = "";
  });

  self = {
    podman-static = (pkgs.podman.overrideAttrs(old: {
      name = "podman-static";
      buildInputs = old.buildInputs ++ (with pkgs; [
        (static pkgs.libassuan)
        (static pkgs.libgpgerror)
        git
        glibc
        glibc.static
      ]);
      src = ./..;
      EXTRA_LDFLAGS = ''-linkmode external -extldflags "-static -lm"'';
      BUILDTAGS = ''static netgo apparmor selinux seccomp systemd varlink containers_image_ostree_stub'';
    })).override {
      gpgme = (static pkgs.gpgme);
      libseccomp = (static pkgs.libseccomp);
      lvm2 = (patchLvm2 (static pkgs.lvm2));
    };
  };
in self
