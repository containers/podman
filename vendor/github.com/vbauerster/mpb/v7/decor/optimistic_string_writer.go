package decor

import "io"

func optimisticStringWriter(w io.Writer) func(string) {
	return func(s string) {
		_, err := io.WriteString(w, s)
		if err != nil {
			panic(err)
		}
	}
}
