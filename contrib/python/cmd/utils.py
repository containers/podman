import sys
import math
import datetime

def write_out(output, lf="\n"):
    _output(sys.stdout, output, lf)


def write_err(output, lf="\n"):
    _output(sys.stderr, output, lf)


def _output(fd, output, lf):
    fd.flush()
    fd.write(output + str(lf))


def convert_size(size):
    if size > 0:
        size_name = ("B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB")
        i = int(math.floor(math.log(size, 1000)))
        p = math.pow(1000, i)
        s = round(size/p, 2) # pylint: disable=round-builtin,old-division
        if s > 0:
            return '%s %s' % (s, size_name[i])
    return '0B'

def stringTimeToHuman(t):
    #datetime.date(datetime.strptime("05/Feb/2016", '%d/%b/%Y'))
    #2018-04-30 13:55:45.019400581 +0000 UTC
    #d = datetime.date(datetime.strptime(t, "%Y-%m-%d"))
    return "sometime ago"
