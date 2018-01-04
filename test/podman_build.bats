#!/usr/bin/env bats

load helpers

@test "build-from-scratch" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} ${BUILDAH_TESTSDIR}/build/from-scratch
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
  [ "$status" -eq 0 ]
}

@test "build-from-multiple-files-one-from" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.scratch -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.nofrom
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  cmp $root/Dockerfile1 ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.scratch
  cmp $root/Dockerfile2.nofrom ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.nofrom
  run test -s $root/etc/passwd
  [ "$status" -ne 0 ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$status" -eq 0 ]
  [ "$output" = "" ]

  target=alpine-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.alpine -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.nofrom
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  cmp $root/Dockerfile1 ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.alpine
  cmp $root/Dockerfile2.nofrom ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.nofrom
  run test -s $root/etc/passwd
  [ "$status" -eq 0 ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "build-from-multiple-files-two-froms" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.scratch -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.withfrom
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  cmp $root/Dockerfile1 ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.scratch
  cmp $root/Dockerfile2.withfrom ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.withfrom
  run test -s $root/etc/passwd
  [ "$status" -ne 0 ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$status" -eq 0 ]
  [ "$output" = "" ]

  target=alpine-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.alpine -f ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.withfrom
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  cmp $root/Dockerfile1 ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.alpine
  cmp $root/Dockerfile2.withfrom ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.withfrom
  run test -s $root/etc/passwd
  [ "$status" -eq 0 ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "build-preserve-subvolumes" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  # This Dockerfile needs us to be able to handle a working RUN instruction.
  if ! which runc ; then
    skip
  fi
  target=volume-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} ${BUILDAH_TESTSDIR}/build/preserve-volumes
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  test -s $root/vol/subvol/subsubvol/subsubvolfile
  run test -s $root/vol/subvol/subvolfile
  [ "$status" -ne 0 ]
  test -s $root/vol/volfile
  test -s $root/vol/Dockerfile
  test -s $root/vol/Dockerfile2
  run test -s $root/vol/anothervolfile
  [ "$status" -ne 0 ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-http-Dockerfile" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  starthttpd ${BUILDAH_TESTSDIR}/build/from-scratch
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f http://0.0.0.0:${HTTP_SERVER_PORT}/Dockerfile .
  stophttpd
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-http-context-with-Dockerfile" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  starthttpd ${BUILDAH_TESTSDIR}/build/http-context
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} http://0.0.0.0:${HTTP_SERVER_PORT}/context.tar
  stophttpd
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-http-context-dir-with-Dockerfile-pre" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  starthttpd ${BUILDAH_TESTSDIR}/build/http-context-subdir
  target=scratch-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f context/Dockerfile http://0.0.0.0:${HTTP_SERVER_PORT}/context.tar
  stophttpd
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-http-context-dir-with-Dockerfile-post" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  starthttpd ${BUILDAH_TESTSDIR}/build/http-context-subdir
  target=scratch-image
  podman build http://0.0.0.0:${HTTP_SERVER_PORT}/context.tar --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f context/Dockerfile
  stophttpd
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-git-context" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  # We need git and ssh to be around to handle cloning a repository.
  if ! which git ; then
    skip
  fi
  if ! which ssh ; then
    skip
  fi
  target=giturl-image
  # Any repo should do, but this one is small and is FROM: scratch.
  gitrepo=git://github.com/projectatomic/nulecule-library
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} "${gitrepo}"
  cid=$(buildah from ${target})
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-github-context" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=github-image
  # Any repo should do, but this one is small and is FROM: scratch.
  gitrepo=github.com/projectatomic/nulecule-library
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} "${gitrepo}"
  cid=$(buildah from ${target})
  podman rm ${cid}
  buildah --debug=false images -q
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-additional-tags" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=scratch-image
  target2=another-scratch-image
  target3=so-many-scratch-images
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -t ${target2} -t ${target3} ${BUILDAH_TESTSDIR}/build/from-scratch
  run buildah --debug=false images
  cid=$(buildah from ${target})
  podman rm ${cid}
  cid=$(buildah from library/${target2})
  podman rm ${cid}
  cid=$(buildah from ${target3}:latest)
  podman rm ${cid}
  podman rmi -f $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-volume-perms" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  # This Dockerfile needs us to be able to handle a working RUN instruction.
  if ! which runc ; then
    skip
  fi
  target=volume-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} ${BUILDAH_TESTSDIR}/build/volume-perms
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  run test -s $root/vol/subvol/subvolfile
  [ "$status" -ne 0 ]
  run stat -c %f $root/vol/subvol
  [ "$output" = 41ed ]
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}

@test "build-from-glob" {
  if ! which buildah ; then
    skip "Buildah not installed"
  fi
  target=alpine-image
  podman build --signature-policy ${BUILDAH_TESTSDIR}/policy.json -t ${target} -f Dockerfile2.glob ${BUILDAH_TESTSDIR}/build/from-multiple-files
  cid=$(buildah from ${target})
  root=$(buildah mount ${cid})
  cmp $root/Dockerfile1.alpine ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile1.alpine
  cmp $root/Dockerfile2.withfrom ${BUILDAH_TESTSDIR}/build/from-multiple-files/Dockerfile2.withfrom
  podman rm ${cid}
  podman rmi $(buildah --debug=false images -q)
  run buildah --debug=false images -q
  [ "$output" = "" ]
}
