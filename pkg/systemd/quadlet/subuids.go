package quadlet

import (
	"os"
	"strconv"
	"strings"
)

// Code to look up subuid/subguid allocations for a user in /etc/subuid and /etc/subgid

func lookupHostSubid(name string, file string, cache *[]string) *Ranges {
	ranges := NewRangesEmpty()

	if len(*cache) == 0 {
		data, e := os.ReadFile(file)
		if e != nil {
			*cache = make([]string, 0)
		} else {
			*cache = strings.Split(string(data), "\n")
		}
		for i := range *cache {
			(*cache)[i] = strings.TrimSpace((*cache)[i])
		}

		// If file had no lines, add an empty line so the above cache created check works
		if len(*cache) == 0 {
			*cache = append(*cache, "")
		}
	}

	for _, line := range *cache {
		if strings.HasPrefix(line, name) &&
			len(line) > len(name)+1 && line[len(name)] == ':' {
			parts := strings.SplitN(line, ":", 3)

			if len(parts) != 3 {
				continue
			}

			start, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				continue
			}

			len, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				continue
			}

			if len > 0 {
				ranges.Add(uint32(start), uint32(len))
			}

			break
		}
	}

	return ranges
}

var subuidCache, subgidCache []string

func lookupHostSubuid(userName string) *Ranges {
	return lookupHostSubid(userName, "/etc/subuid", &subuidCache)
}

func lookupHostSubgid(userName string) *Ranges {
	return lookupHostSubid(userName, "/etc/subgid", &subgidCache)
}
