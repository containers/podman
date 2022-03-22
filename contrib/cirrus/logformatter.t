#!/usr/bin/perl
#
# tests for logformatter
#
(our $ME = $0) =~ s|^.*/||;

use v5.14;
use strict;
use warnings;

use FindBin;
use File::Temp          qw(tempdir);
use Test::More;

# To test links to source files
$ENV{CIRRUS_CHANGE_IN_REPO} = 'ceci-nest-pas-une-sha';

#
# Read the test cases (see __END__ section below)
#
my @tests;
my $context = '';
while (my $line = <DATA>) {
    chomp $line;

    if ($line =~ /^==\s+(.*)/) {
        push @tests, { name => $1, input => [], expect => [] };
        $context = '';
    }
    elsif ($line =~ /^<<</) {
        $context = 'input';
    }
    elsif ($line =~ /^>>>/) {
        $context = 'expect';
    }
    elsif (@tests && $line) {
        push @{ $tests[-1]{$context} }, $line;
    }
}

plan tests => scalar(@tests);

my $tempdir = tempdir("logformatter-test.XXXXXX", TMPDIR => 1, CLEANUP => !$ENV{DEBUG});

chdir $tempdir
    or die "$ME: Could not cd $tempdir: $!\n";

for my $t (@tests) {
    my $name = $t->{name};
    (my $fname = $name) =~ s/\s+/_/g;

    open my $fh_out, '>', "$fname.txt"
        or die "$ME: Cannot create $tempdir/$fname.txt: $!\n";
    print { $fh_out } "$_\n" for @{$t->{input}};
    close $fh_out
        or die "$ME: Error writing $tempdir/$fname.txt: $!\n";

    system("$FindBin::Bin/logformatter $fname <$fname.txt >/dev/null");
    open my $fh_in, '<', "$fname.log.html"
        or die "$ME: Fatal: $fname: logformatter did not create .log.html\n";
    my @actual;
    while (my $line = <$fh_in>) {
        chomp $line;
        push @actual, $line  if $line =~ / begin processed output / .. $line =~ / end processed output /;
    }
    close $fh_in;

    # Strip off leading and trailing "<pre>"
    shift @actual; pop @actual;

    # For debugging: preserve expected results
    if ($ENV{DEBUG}) {
        open my $fh_out, '>', "$fname.expect";
        print { $fh_out } "$_\n" for @{$t->{expect}};
        close $fh_out;
    }

    is_deeply \@actual, $t->{expect}, $name;
}

chdir '/';



__END__

== simple bats

<<<
1..4
ok 1 hi
ok 2 bye # skip no reason
not ok 3 fail
# (from function `assert' in file ./helpers.bash, line 343,
#  from function `expect_output' in file ./helpers.bash, line 370,
#  in test file ./run.bats, line 786)
# $ /path/to/podman foo -bar
# #| FAIL: exit code is 123; expected 321
ok 4 blah
>>>
1..4
<span class='bats-passed'><a name='t--00001'>ok 1 hi</a></span>
<span class='bats-skipped'><a name='t--00002'>ok 2 bye # skip no reason</a></span>
<span class='bats-failed'><a name='t--00003'>not ok 3 fail</a></span>
<span class='bats-log'># (from function `assert&#39; in file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L343">helpers.bash, line 343</a>,</span>
<span class='bats-log'>#  from function `expect_output&#39; in file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L370">helpers.bash, line 370</a>,</span>
<span class='bats-log'>#  in test file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/run.bats#L786">run.bats, line 786</a>)</span>
<span class='bats-log'># $ <b><span title="/path/to/podman">podman</span> foo -bar</b></span>
<span class='bats-log-esm'># #| FAIL: exit code is 123; expected 321</span>
<span class='bats-passed'><a name='t--00004'>ok 4 blah</a></span>
<hr/><span class='bats-summary'>Summary: <span class='bats-passed'>2 Passed</span>, <span class='bats-failed'>1 Failed</span>, <span class='bats-skipped'>1 Skipped</span>. Total tests: 4</span>







== simple ginkgo

<<<
$SCRIPT_BASE/integration_test.sh |& ${TIMESTAMP}
[08:26:19] START - All [+xxxx] lines that follow are relative to right now.
[+0002s] GO111MODULE=on go build -mod=vendor  -gcflags 'all=-trimpath=/var/tmp/go/src/github.com/containers/podman' -asmflags 'all=-trimpath=/var/tmp/go/src/github.com/containers/podman' -ldflags '-X github.com/containers/podman/libpod/define.gitCommit=40f5d8b1becd381c4e8283ed3940d09193e4fe06 -X github.com/containers/podman/libpod/define.buildInfo=1582809981 -X github.com/containers/podman/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/libpod/config._etcDir=/etc -extldflags ""' -tags "   selinux systemd exclude_graphdriver_devicemapper seccomp varlink" -o bin/podman github.com/containers/podman/cmd/podman
[+0103s] •
[+0103s] ------------------------------
[+0103s] Podman pod restart
[+0103s]   podman pod restart single empty pod
[+0103s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_restart_test.go:41
[+0103s] [BeforeEach] Podman pod restart
[+0103s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_restart_test.go:18
[+0103s] [It] podman pod restart single empty pod
[+0103s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_restart_test.go:41
[+0103s] Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --network-backend netavark --storage-opt vfs.imagestore=/tmp/podman/imagecachedir --root /tmp/podman_test553496330/crio --runroot /tmp/podman_test553496330/crio-run --runtime /usr/bin/runc --conmon /usr/bin/conmon --network-config-dir /etc/cni/net.d --cgroup-manager systemd --tmpdir /tmp/podman_test553496330 --events-backend file --storage-driver vfs pod create --infra=false --share
[+0103s] 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89
[+0103s] output: 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89
[+0103s] Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/podman/imagecachedir --root /tmp/podman_test553496330/crio --runroot /tmp/podman_test553496330/crio-run --runtime /usr/bin/runc --conmon /usr/bin/conmon --network-config-dir /etc/cni/net.d --cgroup-manager systemd --tmpdir /tmp/podman_test553496330 --events-backend file --storage-driver vfs pod restart 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89
[+0103s] Error: no containers in pod 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89 have no dependencies, cannot start pod: no such container
[+0103s] output:
[+0103s] [AfterEach] Podman pod restart
[+0103s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_restart_test.go:28
[+0103s] Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/podman/imagecachedir --root /tmp/podman_test553496330/crio --runroot /tmp/podman_test553496330/crio-run --runtime /usr/bin/runc --conmon /usr/bin/conmon --network-config-dir /etc/cni/net.d --cgroup-manager systemd --tmpdir /tmp/podman_test553496330 --events-backend file --storage-driver vfs pod rm -fa
[+0103s] 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89
[+0104s] Running: /var/tmp/go/src/github.com/containers/libpod/bin/podman-remote --storage-opt vfs.imagestore=/tmp/podman/imagecachedir --root /tmp/podman_test553496330/crio --runroot /tmp/podman_test553496330/crio-run --runtime /usr/bin/runc --conmon /usr/bin/conmon --network-config-dir /etc/cni/net.d --cgroup-manager systemd --tmpdir /tmp/podman_test553496330 --events-backend file --storage-driver vfs --remote --url unix:/run/user/12345/podman-xyz.sock pod rm -fa
[+0104s] 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89 again


[+0107s] •
[+0523s] ------------------------------
[+0523s] Podman play kube with build
[+0523s]   --build should override image in store
[+0523s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/play_build_test.go:215


[+0479s] •
[+0479s] ------------------------------
[+0479s] Podman pod rm
[+0479s]   podman pod rm -a doesn't remove a running container
[+0479s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_rm_test.go:119


[+1405s] •
[+1405s] ------------------------------
[+1405s] Podman run entrypoint
[+1405s]   podman run entrypoint == [""]
[+1405s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/run_entrypoint_test.go:47

[+0184s] S [SKIPPING] [3.086 seconds]
[+1385s] S [SKIPPING] in Spec Setup (BeforeEach) [0.001 seconds]

[+1512s] Summarizing 6 Failures:
[+1512s]
[+1512s] [Fail] Podman play kube with build [It] --build should override image in store
[+1512s] /var/tmp/go/src/github.com/containers/podman/test/e2e/play_build_test.go:259
>>>
$SCRIPT_BASE/integration_test.sh |&amp; ${TIMESTAMP}
[08:26:19] START - All [+xxxx] lines that follow are relative to right now.
<span class="timestamp">[+0002s] </span>GO111MODULE=on go build -mod=vendor  -gcflags &#39;all=-trimpath=/var/tmp/go/src/github.com/containers/podman&#39; -asmflags &#39;all=-trimpath=/var/tmp/go/src/github.com/containers/podman&#39; -ldflags &#39;-X github.com/containers/podman/libpod/define.gitCommit=40f5d8b1becd381c4e8283ed3940d09193e4fe06 -X github.com/containers/podman/libpod/define.buildInfo=1582809981 -X github.com/containers/podman/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/libpod/config._etcDir=/etc -extldflags &quot;&quot;&#39; -tags &quot;   selinux systemd exclude_graphdriver_devicemapper seccomp varlink&quot; -o bin/podman github.com/containers/podman/cmd/podman
<span class="timestamp">[+0103s] </span>•
</pre>
<hr />
<pre>
<span class="timestamp">[+0103s] </span>Podman pod restart
<span class="timestamp">         </span><a name='t--podman-pod-restart-single-empty-pod--1'><h2>  podman pod restart single empty pod</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/pod_restart_test.go#L41'>/containers/podman/test/e2e/pod_restart_test.go:41</a>
<span class="timestamp">         </span>[BeforeEach] Podman pod restart
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/pod_restart_test.go#L18'>/containers/podman/test/e2e/pod_restart_test.go:18</a>
<span class="timestamp">         </span><span class="testname">[It] podman pod restart single empty pod</span>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/pod_restart_test.go#L41'>/containers/podman/test/e2e/pod_restart_test.go:41</a>
<span class="timestamp">         </span><span class="boring">#</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--network-backend netavark
--storage-opt vfs.imagestore=/tmp/podman/imagecachedir
--root /tmp/podman_test553496330/crio
--runroot /tmp/podman_test553496330/crio-run
--runtime /usr/bin/runc
--conmon /usr/bin/conmon
--network-config-dir /etc/cni/net.d
--cgroup-manager systemd
--tmpdir /tmp/podman_test553496330
--events-backend file
--storage-driver vfs">[options]</span><b> pod create --infra=false --share</b>
<span class="timestamp">         </span>4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89
<span class="timestamp">         </span><span class="boring">#</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/podman/imagecachedir
--root /tmp/podman_test553496330/crio
--runroot /tmp/podman_test553496330/crio-run
--runtime /usr/bin/runc
--conmon /usr/bin/conmon
--network-config-dir /etc/cni/net.d
--cgroup-manager systemd
--tmpdir /tmp/podman_test553496330
--events-backend file
--storage-driver vfs">[options]</span><b> pod restart 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89</b>
<span class="timestamp">         </span><span class='log-warn'>Error: no containers in pod 4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89 have no dependencies, cannot start pod: no such container</span>
<span class="timestamp">         </span>output:
<span class="timestamp">         </span>[AfterEach] Podman pod restart
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/pod_restart_test.go#L28'>/containers/podman/test/e2e/pod_restart_test.go:28</a>
<span class="timestamp">         </span><span class="boring">#</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/podman/imagecachedir
--root /tmp/podman_test553496330/crio
--runroot /tmp/podman_test553496330/crio-run
--runtime /usr/bin/runc
--conmon /usr/bin/conmon
--network-config-dir /etc/cni/net.d
--cgroup-manager systemd
--tmpdir /tmp/podman_test553496330
--events-backend file
--storage-driver vfs">[options]</span><b> pod rm -fa</b>
<span class="timestamp">         </span>4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89

<span class="timestamp">[+0104s] </span><span class="boring">#</span> <span title="/var/tmp/go/src/github.com/containers/libpod/bin/podman-remote"><b>podman-remote</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/podman/imagecachedir
--root /tmp/podman_test553496330/crio
--runroot /tmp/podman_test553496330/crio-run
--runtime /usr/bin/runc
--conmon /usr/bin/conmon
--network-config-dir /etc/cni/net.d
--cgroup-manager systemd
--tmpdir /tmp/podman_test553496330
--events-backend file
--storage-driver vfs
--url unix:/run/user/12345/podman-xyz.sock">[options]</span><b> pod rm -fa</b>
<span class="timestamp">         </span>4810be0cfbd42241e349dbe7d50fbc54405cd320a6637c65fd5323f34d64af89 again


<span class="timestamp">[+0107s] </span>•
</pre>
<hr />
<pre>
<span class="timestamp">[+0523s] </span>Podman play kube with build
<span class="timestamp">         </span><a name='t----build-should-override-image-in-store--1'><h2>  --build should override image in store</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/play_build_test.go#L215'>/containers/podman/test/e2e/play_build_test.go:215</a>


<span class="timestamp">[+0479s] </span>•
</pre>
<hr />
<pre>
<span class="timestamp">[+0479s] </span>Podman pod rm
<span class="timestamp">         </span><a name='t--podman-pod-rm--a-doesnt-remove-a-running-container--1'><h2>  podman pod rm -a doesn&#39;t remove a running container</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/pod_rm_test.go#L119'>/containers/podman/test/e2e/pod_rm_test.go:119</a>


<span class="timestamp">[+1405s] </span>•
</pre>
<hr />
<pre>
<span class="timestamp">[+1405s] </span>Podman run entrypoint
<span class="timestamp">         </span><a name='t--podman-run-entrypoint---1'><h2>  podman run entrypoint == [&quot;&quot;]</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/run_entrypoint_test.go#L47'>/containers/podman/test/e2e/run_entrypoint_test.go:47</a>


<span class="timestamp">[+0184s] </span>S <span class="log-skip">[SKIPPING] [3.086 seconds]</span>
<span class="timestamp">[+1385s] </span>S <span class="log-skip">[SKIPPING] in Spec Setup (BeforeEach) [0.001 seconds]</span>


<span class="timestamp">[+1512s] </span>Summarizing 6 Failures:
[+1512s]
<span class="timestamp">         </span><b>[Fail] Podman play kube with build [It] <a href='#t----build-should-override-image-in-store--1'>--build should override image in store</a></b>
<span class="timestamp">         </span>/var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/40f5d8b1becd381c4e8283ed3940d09193e4fe06/test/e2e/play_build_test.go#L259'>/containers/podman/test/e2e/play_build_test.go:259</a>


== simple python

<<<
[+0234s] env CONTAINERS_CONF=/var/tmp/go/src/github.com/containers/podman/test/apiv2/containers.conf PODMAN=./bin/podman /usr/bin/python3 -m unittest discover -v ./test/python/docker
[+0238s] test_copy_to_container (compat.test_containers.TestContainers) ... /usr/lib/python3.10/site-packages/docker/utils/utils.py:269: DeprecationWarning: urllib.parse.splitnport() is deprecated as of 3.8, use urllib.parse.urlparse() instead
[+0238s]   host, port = splitnport(parsed_url.netloc)
[+0241s] ok
[+0243s] test_create_container (compat.test_containers.TestContainers) ... ok
[+0244s] test_create_network (compat.test_containers.TestContainers) ... ok
[+0245s] test_filters (compat.test_containers.TestContainers) ... skipped 'TODO Endpoint does not yet support filters'
[+0246s] test_kill_container (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/threading.py:372: ResourceWarning: unclosed <socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55054), raddr=('127.0.0.1', 8080)>
[+0246s]   waiters_to_notify = _deque(_islice(all_waiters, n))
[+0246s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0247s] ok
[+0248s] test_list_container (compat.test_containers.TestContainers) ... ok
[+0252s] test_mount_preexisting_dir (compat.test_containers.TestContainers) ... ok
[+0253s] test_mount_rw_by_default (compat.test_containers.TestContainers) ... ok
[+0257s] test_non_existant_workdir (compat.test_containers.TestContainers) ... ok
[+0258s] test_pause_container (compat.test_containers.TestContainers) ... ok
[+0260s] test_pause_stopped_container (compat.test_containers.TestContainers) ... ok
[+0261s] test_remove_container (compat.test_containers.TestContainers) ... ok
[+0262s] test_remove_container_without_force (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed <socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55068), raddr=('127.0.0.1', 8080)>
[+0262s]   for ateof in reversed(self._eofstack):
[+0262s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0262s] /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed <socket.socket fd=5, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55074), raddr=('127.0.0.1', 8080)>
[+0262s]   for ateof in reversed(self._eofstack):
[+0262s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0262s] ok
[+0264s] test_restart_container (compat.test_containers.TestContainers) ... ok
[+0265s] test_start_container (compat.test_containers.TestContainers) ... ok
[+0267s] test_start_container_with_random_port_bind (compat.test_containers.TestContainers) ... ok
[+0268s] test_stop_container (compat.test_containers.TestContainers) ... ok
[+0269s] test_unpause_container (compat.test_containers.TestContainers) ... ok
[+0273s] test_build_image (compat.test_images.TestImages) ... ok
[+0273s] test_get_image_exists_not (compat.test_images.TestImages)
[+0274s] Negative test for get image ... ok
[+0274s] test_image_history (compat.test_images.TestImages)
[+0274s] Image history ... ok
[+0274s] test_list_images (compat.test_images.TestImages)
[+0276s] List images ... ok
[+0276s] test_load_corrupt_image (compat.test_images.TestImages)
[+0277s] Import|Load Image failure ... ok
[+0277s] test_load_image (compat.test_images.TestImages)
[+0279s] Import|Load Image ... ok
[+0279s] test_remove_image (compat.test_images.TestImages)
[+0280s] Remove image ... ok
[+0280s] test_retag_valid_image (compat.test_images.TestImages)
[+0280s] Validates if name updates when the image is retagged ... ok
[+0280s] test_save_image (compat.test_images.TestImages)
[+0282s] Export Image ... ok
[+0282s] test_search_bogus_image (compat.test_images.TestImages)
[+0290s] Search for bogus image should throw exception ... ok
[+0290s] test_search_image (compat.test_images.TestImages)
[+0291s] Search for image ... FAIL
[+0291s] test_tag_valid_image (compat.test_images.TestImages)
[+0292s] Validates if the image is tagged successfully ... ok
[+0296s] test_Info (compat.test_system.TestSystem) ... ok
[+0298s] test_info_container_details (compat.test_system.TestSystem) ... ok
[+0299s] test_version (compat.test_system.TestSystem) ... ok
[+0299s] ======================================================================
[+0299s] FAIL: test_search_image (compat.test_images.TestImages)
[+0299s] Search for image
[+0299s] ----------------------------------------------------------------------
[+0299s] Traceback (most recent call last):
[+0299s]   File "/var/tmp/go/src/github.com/containers/podman/test/python/docker/compat/test_images.py", line 90, in test_search_image
[+0299s]     self.assertIn("alpine", r["Name"])
[+0299s] AssertionError: 'alpine' not found in 'docker.io/docker/desktop-kubernetes'
[+0299s] ----------------------------------------------------------------------
[+0299s] Ran 33 tests in 63.138s
[+0299s] FAILED (failures=1, skipped=1)
[+0299s] make: *** [Makefile:616: localapiv2] Error 1
>>>
<span class="timestamp">[+0234s] </span>env CONTAINERS_CONF=/var/tmp/go/src/github.com/containers/podman/test/apiv2/containers.conf PODMAN=./bin/podman /usr/bin/python3 -m unittest discover -v ./test/python/docker
<span class="timestamp">[+0238s] </span>test_copy_to_container (compat.test_containers.TestContainers) ... /usr/lib/python3.10/site-packages/docker/utils/utils.py:269: DeprecationWarning: urllib.parse.splitnport() is deprecated as of 3.8, use urllib.parse.urlparse() instead
<span class="timestamp">         </span>  host, port = splitnport(parsed_url.netloc)
<span class="timestamp">[+0241s] </span>ok
<span class="timestamp">[+0243s] </span><span class='bats-passed'>test_create_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0244s] </span><span class='bats-passed'>test_create_network (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0245s] </span><span class='bats-skipped'>test_filters (compat.test_containers.TestContainers) ... skipped &#39;TODO Endpoint does not yet support filters&#39;</span>
<span class="timestamp">[+0246s] </span>test_kill_container (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/threading.py:372: ResourceWarning: unclosed &lt;socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55054), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  waiters_to_notify = _deque(_islice(all_waiters, n))
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">[+0247s] </span>ok
<span class="timestamp">[+0248s] </span><span class='bats-passed'>test_list_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0252s] </span><span class='bats-passed'>test_mount_preexisting_dir (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0253s] </span><span class='bats-passed'>test_mount_rw_by_default (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0257s] </span><span class='bats-passed'>test_non_existant_workdir (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0258s] </span><span class='bats-passed'>test_pause_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0260s] </span><span class='bats-passed'>test_pause_stopped_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0261s] </span><span class='bats-passed'>test_remove_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0262s] </span>test_remove_container_without_force (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed &lt;socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55068), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  for ateof in reversed(self._eofstack):
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">         </span>/usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed &lt;socket.socket fd=5, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55074), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  for ateof in reversed(self._eofstack):
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">         </span>ok
<span class="timestamp">[+0264s] </span><span class='bats-passed'>test_restart_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0265s] </span><span class='bats-passed'>test_start_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0267s] </span><span class='bats-passed'>test_start_container_with_random_port_bind (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0268s] </span><span class='bats-passed'>test_stop_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0269s] </span><span class='bats-passed'>test_unpause_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0273s] </span><span class='bats-passed'>test_build_image (compat.test_images.TestImages) ... ok</span>
<span class="timestamp">         </span>test_get_image_exists_not (compat.test_images.TestImages)
<span class="timestamp">[+0274s] </span><span class='bats-passed'>Negative test for get image ... ok</span>
<span class="timestamp">         </span>test_image_history (compat.test_images.TestImages)
<span class="timestamp">         </span><span class='bats-passed'>Image history ... ok</span>
<span class="timestamp">         </span>test_list_images (compat.test_images.TestImages)
<span class="timestamp">[+0276s] </span><span class='bats-passed'>List images ... ok</span>
<span class="timestamp">         </span>test_load_corrupt_image (compat.test_images.TestImages)
<span class="timestamp">[+0277s] </span><span class='bats-passed'>Import|Load Image failure ... ok</span>
<span class="timestamp">         </span>test_load_image (compat.test_images.TestImages)
<span class="timestamp">[+0279s] </span><span class='bats-passed'>Import|Load Image ... ok</span>
<span class="timestamp">         </span>test_remove_image (compat.test_images.TestImages)
<span class="timestamp">[+0280s] </span><span class='bats-passed'>Remove image ... ok</span>
<span class="timestamp">         </span>test_retag_valid_image (compat.test_images.TestImages)
<span class="timestamp">         </span><span class='bats-passed'>Validates if name updates when the image is retagged ... ok</span>
<span class="timestamp">         </span>test_save_image (compat.test_images.TestImages)
<span class="timestamp">[+0282s] </span><span class='bats-passed'>Export Image ... ok</span>
<span class="timestamp">         </span>test_search_bogus_image (compat.test_images.TestImages)
<span class="timestamp">[+0290s] </span><span class='bats-passed'>Search for bogus image should throw exception ... ok</span>
<span class="timestamp">         </span>test_search_image (compat.test_images.TestImages)
<span class="timestamp">[+0291s] </span><span class='bats-failed'>Search for image ... FAIL</span>
<span class="timestamp">         </span>test_tag_valid_image (compat.test_images.TestImages)
<span class="timestamp">[+0292s] </span><span class='bats-passed'>Validates if the image is tagged successfully ... ok</span>
<span class="timestamp">[+0296s] </span><span class='bats-passed'>test_Info (compat.test_system.TestSystem) ... ok</span>
<span class="timestamp">[+0298s] </span><span class='bats-passed'>test_info_container_details (compat.test_system.TestSystem) ... ok</span>
<span class="timestamp">[+0299s] </span><span class='bats-passed'>test_version (compat.test_system.TestSystem) ... ok</span>
<div class='log-error'>
<span class="timestamp">         </span>======================================================================
<span class="timestamp">         </span>FAIL: test_search_image (compat.test_images.TestImages)
<span class="timestamp">         </span>Search for image
<span class="timestamp">         </span>----------------------------------------------------------------------
<span class="timestamp">         </span>Traceback (most recent call last):
<span class="timestamp">         </span>  File &quot;/var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/python/docker/compat/test_images.py#L90'>/containers/podman/test/python/docker/compat/test_images.py&quot;, line 90</a>, in test_search_image
<span class="timestamp">         </span>    self.assertIn(&quot;alpine&quot;, r[&quot;Name&quot;])
<span class="timestamp">         </span>AssertionError: &#39;alpine&#39; not found in &#39;docker.io/docker/desktop-kubernetes&#39;
<span class="timestamp">         </span>----------------------------------------------------------------------
</div>
<span class="timestamp">         </span>Ran 33 tests in 63.138s
<span class="timestamp">         </span><span class='bats-failed'>FAILED (failures=1, skipped=1)</span>
<span class="timestamp">         </span>make: *** [Makefile:616: localapiv2] Error 1
<hr/><span class='bats-summary'>Summary: <span class='bats-passed'>28 Passed</span>, <span class='bats-failed'>1 Failed</span>, <span class='bats-skipped'>1 Skipped</span>. Total tests: 30 <span class='bats-failed'>(WARNING: expected 33)</span></span>
