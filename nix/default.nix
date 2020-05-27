{ system ? builtins.currentSystem }:
let
  pkgs = (import ./nixpkgs.nix {
    config = {
      packageOverrides = pkg: {
        gpgme = (static pkg.gpgme);
        libassuan = (static pkg.libassuan);
        libgpgerror = (static pkg.libgpgerror);
        libseccomp = (static pkg.libseccomp);
        systemd = pkg.systemd.overrideAttrs(x: {
          mesonFlags = x.mesonFlags ++ [ "-Dstatic-libsystemd=true" ];
          postFixup = ''
            ${x.postFixup}
            sed -ri "s;$out/(.*);$nukedRef/\1;g" $lib/lib/libsystemd.a
          '';
        });
      };
    };
  });

  static = pkg: pkg.overrideAttrs(x: {
    configureFlags = (x.configureFlags or []) ++
      [ "--without-shared" "--disable-shared" ];
    dontDisableStatic = true;
    enableSharedExecutables = false;
    enableStatic = true;
  });

  self = with pkgs; buildGoPackage rec {
    name = "podman";
    src = ./..;
    goPackagePath = "github.com/containers/libpod";
    doCheck = false;
    enableParallelBuilding = true;
    nativeBuildInputs = [ git go-md2man installShellFiles pkg-config ];
    buildInputs = [ glibc glibc.static gpgme libapparmor libassuan libgpgerror libseccomp libselinux systemd ];
    prePatch = ''
      export LDFLAGS='-static-libgcc -static -s -w'
      export EXTRA_LDFLAGS='-s -w -linkmode external -extldflags "-static -lm"'
      export BUILDTAGS='static netgo varlink apparmor selinux seccomp systemd exclude_graphdriver_btrfs exclude_graphdriver_devicemapper'
    '';
    buildPhase = ''
      pushd go/src/${goPackagePath}
      patchShebangs .
      make bin/podman
    '';
    installPhase = ''
      install -Dm755 bin/podman $out/bin/podman
    '';
  };
in self
