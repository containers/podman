package pods

import (
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
)

// readPodIDFiles reads the specified files and returns their content (i.e.,
// first line).
func readPodIDFiles(files []string) ([]string, error) {
	ids := []string{}
	for _, podFile := range files {
		content, err := ioutil.ReadFile(podFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading pod ID file")
		}
		id := strings.Split(string(content), "\n")[0]
		ids = append(ids, id)
	}
	return ids, nil
}
