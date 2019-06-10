

# This script is intended to be piped into by automation, in order to
# mark output lines with timing information.  For example:
#      /path/to/command |& awk --file timestamp.awk

BEGIN {
    STARTTIME=systime()
    printf "[%s] START", strftime("%T")
    printf " - All [+xxxx] lines that follow are relative to right now.\n"
}

{
    printf "[%+05ds] %s\n", systime()-STARTTIME, $0
}

END {
    printf "[%s] END", strftime("%T")
    printf " - [%+05ds] total duration since START\n", systime()-STARTTIME
}
