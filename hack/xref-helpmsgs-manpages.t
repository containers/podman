# -*- perl -*-
#
# tests for xref-helpmsgs-manpages
#

use v5.20;
use strict;
use warnings;

use Clone                       qw(clone);
use File::Basename;
use File::Path                  qw(make_path);
use File::Temp                  qw(tempdir);
use FindBin;
use Test::More;
use Test::Differences;

plan tests => 9;

require_ok "$FindBin::Bin/xref-helpmsgs-manpages";


my $workdir = tempdir(basename($0) . ".XXXXXXXX", TMPDIR => 1, CLEANUP => 1);

# Copy man pages to tmpdir, so we can fiddle with them
my $doc_path = do {
    no warnings 'once';
    "$LibPod::CI::XrefHelpmsgsManpages::Markdown_Path";
};

make_path "$workdir/$doc_path"
    or die "Internal error: could not make_path $workdir/$doc_path";
system('rsync', '-a', "$doc_path/." => "$workdir/$doc_path/.") == 0
    or die "Internal error: could not rsync $doc_path to $workdir";

chdir $workdir
    or die "Internal error: could not cd $workdir: $!";

my @warnings_seen;
$SIG{__WARN__} = sub {
    my $msg = shift;
    chomp $msg;
    $msg =~ s/^xref-\S+?:\s+//;
    $msg =~ s|\s+$doc_path/| |g;
    push @warnings_seen, $msg;
};

# When we get errors (hopefully only when adding new functionality
# to this test!), this format is MUCH easier to read and copy-paste.
unified_diff;

# Helper function for running xref tests.
sub test_xref {
    my $name = shift;
    my $h = shift;
    my $m = shift;
    my $expect_by_help = shift;
    my $expect_by_man  = shift;

    @warnings_seen = ();
    LibPod::CI::XrefHelpmsgsManpages::xref_by_help($h, $m);
    eq_or_diff_text \@warnings_seen, $expect_by_help, "$name: xref_by_help()";

    @warnings_seen = ();
    LibPod::CI::XrefHelpmsgsManpages::xref_by_man($h, $m);
    eq_or_diff_text \@warnings_seen, $expect_by_man, "$name: xref_by_man()";
}

###############################################################################
# BEGIN Baseline tests
#
# Confirm that everything passes in the current tree

my $help = LibPod::CI::XrefHelpmsgsManpages::podman_help();
eq_or_diff_text \@warnings_seen, [], "podman_help() runs cleanly, no warnings";

@warnings_seen = ();
my $man = LibPod::CI::XrefHelpmsgsManpages::podman_man('podman');
eq_or_diff_text \@warnings_seen, [], "podman_man() runs cleanly, no warnings";

# If this doesn't pass, we've got big problems.
test_xref("baseline", $help, $man, [], []);

#use Data::Dump; dd $man; exit 0;

# END   Baseline tests
##########################################################################
# BEGIN fault injection tests on xref_by_man()
#
# These are really simple: only two different warnings.

my $hclone = clone($help);
my $mclone = clone($man);

delete $hclone->{network}{ls}{"--format"};
delete $hclone->{save};
$mclone->{"command-in-man"} = {};
$mclone->{"system"}{"subcommand-in-man"} = {};

# --format field documented in man page but not in autocomplete
delete $hclone->{events}{"--format"}{".HealthStatus"};

test_xref("xref_by_man injection", $hclone, $mclone,
          [],
          [
              "'podman ': 'command-in-man' in podman.1.md, but not in --help",
              "'podman events --format': '.HealthStatus' in podman-events.1.md, but not in command completion",
              "'podman network ls': --format options documented in man page, but not available via autocomplete",
              "'podman ': 'save' in podman.1.md, but not in --help",
              "'podman system': 'subcommand-in-man' in podman-system.1.md, but not in --help",
          ],
      );

# END   fault injection tests on xref_by_man()
###############################################################################
# BEGIN fault injection tests on xref_by_help()
#
# These are much more complicated.

$hclone = clone($help);
$mclone = clone($man);

# --format is not documented in man page
delete $mclone->{"auto-update"}{"--format"};
# --format is documented, but without a table
$mclone->{container}{list}{"--format"} = 1;
# --format is documented, with a table, but one entry missing
delete $mclone->{events}{"--format"}{".HealthStatus"};

# -l option is not documented
delete $mclone->{pod}{inspect}{"-l"};

# Command and subcommand in podman --help, but not in man pages
$hclone->{"new-command-in-help"} = {};
$hclone->{"secret"}{"subcommand-in-help"} = {};

# Can happen if podman-partlydocumented exists in --help, and is listed
# in podman.1.md, but does not have its own actual man page.
$hclone->{partlydocumented} = { "something" => 1 };
$mclone->{partlydocumented} = undef;

test_xref("xref_by_help() injection", $hclone, $mclone,
          [
              "'podman auto-update --help' lists '--format', which is not in podman-auto-update.1.md",
              "'podman container list': --format options are available through autocomplete, but are not documented in podman-ps.1.md",
              "'podman events --format <TAB>' lists '.HealthStatus', which is not in podman-events.1.md",
              "'podman  --help' lists 'new-command-in-help', which is not in podman.1.md",
              "'podman partlydocumented' is not documented in man pages!",
              "'podman pod inspect --help' lists '-l', which is not in podman-pod-inspect.1.md",
              "'podman secret --help' lists 'subcommand-in-help', which is not in podman-secret.1.md",
          ],
          [],
      );

# END   fault injection tests on xref_by_help()
###############################################################################
# BEGIN fault injection tests on podman_man()
#
# This function has a ton of sanity checks. To test them we need to
# perform minor surgery on lots of .md files.
#
# FIXME: TBD. This PR has grown big enough as it is.


# END   fault injection tests on podman_man()
###############################################################################
# BEGIN fault injection tests on podman_help()
#
# Nope, this is not likely to happen. In order to do this we'd need to:
#
#   * instrument podman and cobra to emit fake output; or
#   * write a podman wrapper that selectively munges output; or
#   * write a dummy podman that generates the right form of (broken) output.
#
# podman_help() has few sanity checks, and those are unlikely, so doing this
# is way more effort than it's worth.
#
# END   fault injection tests on podman_help()
###############################################################################

1;
