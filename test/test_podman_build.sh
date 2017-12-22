#!/bin/bash
#
# test_podman_build.sh
#
# Used to test 'podman build' functionality "by hand"
# until we're able to install Buildah in the Travis CI
# test system.
#
# Requires podman and Buildah to be installed on the
# system.  This needs to be run from the libpod
# directory after cloning the libpod repo.
#
# To run:
#   /bin/bash -v test_podman_build.sh
#

HOME=`pwd`

########
# test "build-from-scratch"
########
  TARGET=scratch-image
  podman build -q=True -t $TARGET $HOME/test/build/from-scratch
  CID=$(buildah from $TARGET)
  buildah rm $CID
  podman build -q=False --build-arg HOME=/ --build-arg VERSION=0.1 -t $TARGET $HOME/test/build/from-scratch
  CID=$(buildah from $TARGET)
  buildah rm $CID
  podman build --quiet=True -t $TARGET $HOME/test/build/from-scratch
  CID=$(buildah from $TARGET)
  buildah rm $CID
  podman rmi -f $(podman images -q)
  podman images -q


########
# test "build-preserve-subvolumes"
########
  TARGET=volume-image
  podman build -t $TARGET $HOME/test/build/preserve-volumes
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  test -s $ROOT/vol/subvol/subsubvol/subsubvolfile
  test -s $ROOT/vol/subvol/subvolfile
  test -s $ROOT/vol/volfile
  test -s $ROOT/vol/Dockerfile
  test -s $ROOT/vol/Dockerfile2
  test -s $ROOT/vol/anothervolfile
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  buildah --debug=false images -q

########
# test "build-git-context"
########
  TARGET=giturl-image
  # Any repo should do, but this one is small and is FROM: scratch.
  GITREPO=git://github.com/projectatomic/nulecule-library
  podman build -t $TARGET "$GITREPO"
  CID=$(buildah from $TARGET)
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  podman images -q


########
# test "build-github-context"
########
  TARGET=github-image
  # Any repo should do, but this one is small and is FROM: scratch.
  GITREPO=github.com/projectatomic/nulecule-library
  podman build -t $TARGET "$GITREPO"
  CID=$(buildah from $TARGET)
  buildah rm $CID
  buildah --debug=false images -q
  podman rmi $(buildah --debug=false images -q)
  podman images -q


########
# test "build-additional-tags"
########
  TARGET=scratch-image
  TARGET2=another-scratch-image
  TARGET3=so-many-scratch-images
  podman build -t $TARGET -t $TARGET2 -t $TARGET3 -f $HOME/test/build/from-scratch/Dockerfile
  buildah --debug=false images
  CID=$(buildah from $TARGET)
  buildah rm $CID
  CID=$(buildah from library/$TARGET2)
  buildah rm $CID
  CID=$(buildah from $TARGET3:latest)
  buildah rm $CID
  podman rmi -f $(buildah --debug=false images -q)
  podman images -q


########
# test "build-volume-perms"
########
  TARGET=volume-image
  podman build -t $TARGET $HOME/test/build/volume-perms
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  test -s $ROOT/vol/subvol/subvolfile
  stat -c %f $ROOT/vol/subvol
  #Output s/b 41ed
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  podman images -q


########
# test "build-from-glob"
########
  TARGET=alpine-image
  podman build -t $TARGET -file Dockerfile2.glob $HOME/test/build/from-multiple-files
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  cmp $ROOT/Dockerfile1.alpine $HOME/test/build/from-multiple-files/Dockerfile1.alpine
  cmp $ROOT/Dockerfile2.withfrom $HOME/test/build/from-multiple-files/Dockerfile2.withfrom
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  podman images -q


########
# test "build-from-multiple-files-one-from"
########
  TARGET=scratch-image
  podman build -t $TARGET -file $HOME/test/build/from-multiple-files/Dockerfile1.scratch -file $HOME/test/build/from-multiple-files/Dockerfile2.nofrom
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  cmp $ROOT/Dockerfile1 $HOME/test/build/from-multiple-files/Dockerfile1.scratch
  cmp $ROOT/Dockerfile2.nofrom $HOME/test/build/from-multiple-files/Dockerfile2.nofrom
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  buildah --debug=false images -q

  TARGET=alpine-image
  podman build -t $TARGET -file $HOME/test/build/from-multiple-files/Dockerfile1.alpine -file $HOME/test/build/from-multiple-files/Dockerfile2.nofrom
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  buildah --debug=false images -q


########
# test "build-from-multiple-files-two-froms"
########
  TARGET=scratch-image
  podman build -t $TARGET -file $HOME/test/build/from-multiple-files/Dockerfile1.scratch -file $HOME/test/build/from-multiple-files/Dockerfile2.withfrom
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  cmp $ROOT/Dockerfile1 $HOME/test/build/from-multiple-files/Dockerfile1.scratch
  cmp $ROOT/Dockerfile2.withfrom $HOME/test/build/from-multiple-files/Dockerfile2.withfrom
  test -s $ROOT/etc/passwd
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  buildah --debug=false images -q

  TARGET=alpine-image
  podman build -t $TARGET -file $HOME/test/build/from-multiple-files/Dockerfile1.alpine -file $HOME/test/build/from-multiple-files/Dockerfile2.withfrom
  CID=$(buildah from $TARGET)
  ROOT=$(buildah mount $CID)
  cmp $ROOT/Dockerfile1 $HOME/test/build/from-multiple-files/Dockerfile1.alpine
  cmp $ROOT/Dockerfile2.withfrom $HOME/test/build/from-multiple-files/Dockerfile2.withfrom
  test -s $ROOT/etc/passwd
  buildah rm $CID
  podman rmi $(buildah --debug=false images -q)
  buildah --debug=false images -q
