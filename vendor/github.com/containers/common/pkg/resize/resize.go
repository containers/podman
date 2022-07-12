/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resize

// TerminalSize represents the width and height of a terminal.
type TerminalSize struct {
	Width  uint16
	Height uint16
}

// HandleResizing spawns a goroutine that processes the resize channel, calling resizeFunc for each
// TerminalSize received from the channel. The resize channel must be closed elsewhere to stop the
// goroutine.
func HandleResizing(resize <-chan TerminalSize, resizeFunc func(size TerminalSize)) {
	if resize == nil {
		return
	}

	go func() {
		for {
			size, ok := <-resize
			if !ok {
				return
			}
			if size.Height < 1 || size.Width < 1 {
				continue
			}
			resizeFunc(size)
		}
	}()
}
