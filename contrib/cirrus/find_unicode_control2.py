#!/usr/bin/env python3
"""Find unicode control characters in source files

By default the script takes one or more files or directories and looks for
unicode control characters in all text files.  To narrow down the files, provide
a config file with the -c command line, defining a scan_exclude list, which
should be a list of regular expressions matching paths to exclude from the scan.

There is a second mode enabled with -p which when set to 'all', prints all
control characters and when set to 'bidi', prints only the 9 bidirectional
control characters.
"""
from __future__ import print_function

import sys, os, argparse, re, unicodedata, subprocess
import importlib
from stat import *

def _unicode(line, encoding):
    if isinstance(line, str):
        return line
    return line.decode(encoding)

import platform
if platform.python_version()[0] == '2':
    _chr = unichr
    do_unicode = unicode
else:
    _chr = chr
    do_unicode = _unicode

scan_exclude = [r'\.git/', r'\.hg/', r'\.desktop$', r'ChangeLog$', r'NEWS$',
                r'\.ppd$', r'\.txt$', r'\.directory$']
scan_exclude_mime = [r'text/x-po$', r'text/x-tex$', r'text/x-troff$',
                     r'text/html$']
verbose_mode = False

# Print to stderr in verbose mode.
def eprint(*args, **kwargs):
    if verbose_mode:
        print(*args, file=sys.stderr, **kwargs)

# Decode a single latin1 line.
def decodeline(inf):
    return do_unicode(inf, 'utf-8')

# Make a text string from a file, attempting to decode from latin1 if necessary.
# Other non-utf-8 locales are not supported at the moment.
def getfiletext(filename):
    text = None
    with open(filename) as infile:
        try:
            if detailed_mode:
                return [decodeline(inf) for inf in infile]
        except Exception as e:
            eprint('%s: %s' % (filename, e))
            return None

        try:
            text = decodeline(''.join(infile))
        except UnicodeDecodeError:
            eprint('%s: Retrying with latin1' % filename)
            try:
                text = ''.join([decodeline(inf) for inf in infile])
            except Exception as e:
                eprint('%s: %s' % (filename, e))
    if text:
        return set(text)
    else:
        return None

def analyze_text_detailed(filename, text, disallowed, msg):
    line = 0
    warned = False
    for t in text:
        line = line + 1
        subset = [c for c in t if chr(ord(c)) in disallowed]
        if subset:
            print('%s:%d %s: %s' % (filename, line, msg, subset))
            warned = True
    if not warned:
        eprint('%s: OK' % filename)

# Look for disallowed characters in the text.  We reduce all characters into a
# set to speed up analysis.  FIXME: Add a slow mode to get line numbers in files
# that have these disallowed chars.
def analyze_text(filename, text, disallowed, msg):
    if detailed_mode:
        analyze_text_detailed(filename, text, disallowed, msg)
        return

    if not text.isdisjoint(disallowed):
        print('%s: %s: %s' % (filename, msg, text & disallowed))
    else:
        eprint('%s: OK' % filename)

def should_read(f):
    args = ['file', '--mime-type', f]
    proc = subprocess.Popen(args, stdout=subprocess.PIPE)
    m = [decodeline(x[:-1]) for x in proc.stdout][0].split(':')[1].strip()
    # Fast check, just the file name.
    if [e for e in scan_exclude if re.search(e, f)]:
        return False

    # Slower check, mime type.
    if not 'text/' in m \
            or [e for e in scan_exclude_mime if re.search(e, m)]:
        return False
    return True

# Get file text and feed into analyze_text.
def analyze_file(f, disallowed, msg):
    eprint('%s: Reading file' % f)
    if should_read(f):
        text = getfiletext(f)
        if text:
            analyze_text(f, text, disallowed, msg)
    else:
        eprint('%s: SKIPPED' % f)

# Actual implementation of the recursive descent into directories.
def analyze_any(p, disallowed, msg):
    mode = os.stat(p).st_mode
    if S_ISDIR(mode):
        analyze_dir(p, disallowed, msg)
    elif S_ISREG(mode):
        analyze_file(p, disallowed, msg)
    else:
        eprint('%s: UNREADABLE' % p)

# Recursively analyze files in the directory.
def analyze_dir(d, disallowed, msg):
    for f in os.listdir(d):
        analyze_any(os.path.join(d, f), disallowed, msg)

def analyze_paths(paths, disallowed, msg):
    for p in paths:
        analyze_any(p, disallowed, msg)

# All control characters.  We omit the ascii control characters.
def nonprint_unicode(c):
    cat = unicodedata.category(c)
    if cat.startswith('C') and cat != 'Cc':
        return True
    return False

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Look for Unicode control characters")
    parser.add_argument('path', metavar='path', nargs='+',
            help='Sources to analyze')
    parser.add_argument('-p', '--nonprint', required=False,
            type=str, choices=['all', 'bidi'],
            help='Look for either all non-printable unicode characters or bidirectional control characters.')
    parser.add_argument('-v', '--verbose', required=False, action='store_true',
            help='Verbose mode.')
    parser.add_argument('-d', '--detailed', required=False, action='store_true',
            help='Print line numbers where characters occur.')
    parser.add_argument('-t', '--notests', required=False,
            action='store_true', help='Exclude tests (basically test.* as a component of path).')
    parser.add_argument('-c', '--config', required=False, type=str,
            help='Configuration file to read settings from.')

    args = parser.parse_args()
    verbose_mode = args.verbose
    detailed_mode = args.detailed

    if not args.nonprint:
        # Formatting control characters in the unicode space.  This includes the
        # bidi control characters.
        disallowed = set(_chr(c) for c in range(sys.maxunicode) if \
                                 unicodedata.category(_chr(c)) == 'Cf')

        msg = 'unicode control characters'
    elif args.nonprint == 'all':
        # All control characters.
        disallowed = set(_chr(c) for c in range(sys.maxunicode) if \
                         nonprint_unicode(_chr(c)))

        msg = 'disallowed characters'
    else:
        # Only bidi control characters.
        disallowed = set([
            _chr(0x202a), _chr(0x202b), _chr(0x202c), _chr(0x202d), _chr(0x202e),
            _chr(0x2066), _chr(0x2067), _chr(0x2068), _chr(0x2069)])
        msg = 'bidirectional control characters'

    if args.config:
        spec = importlib.util.spec_from_file_location("settings", args.config)
        settings = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(settings)
        if hasattr(settings, 'scan_exclude'):
            scan_exclude = scan_exclude + settings.scan_exclude
        if hasattr(settings, 'scan_exclude_mime'):
            scan_exclude_mime = scan_exclude_mime + settings.scan_exclude_mime

    if args.notests:
        scan_exclude = scan_exclude + [r'/test[^/]+/']

    analyze_paths(args.path, disallowed, msg)
