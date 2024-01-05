package regexp

import (
	"io"
	"regexp"
	"sync"
)

// Regexp is a wrapper struct used for wrapping MustCompile regex expressions
// used as global variables. Using this structure helps speed the startup time
// of apps that want to use global regex variables. This library initializes them on
// first use as opposed to the start of the executable.
type Regexp struct {
	once   sync.Once
	regexp *regexp.Regexp
	val    string
}

func Delayed(val string) Regexp {
	re := Regexp{
		val: val,
	}
	if precompile {
		re.regexp = regexp.MustCompile(re.val)
	}
	return re
}

func (re *Regexp) compile() {
	if precompile {
		return
	}
	re.once.Do(func() {
		re.regexp = regexp.MustCompile(re.val)
	})
}

func (re *Regexp) Expand(dst []byte, template []byte, src []byte, match []int) []byte {
	re.compile()
	return re.regexp.Expand(dst, template, src, match)
}

func (re *Regexp) ExpandString(dst []byte, template string, src string, match []int) []byte {
	re.compile()
	return re.regexp.ExpandString(dst, template, src, match)
}
func (re *Regexp) Find(b []byte) []byte {
	re.compile()
	return re.regexp.Find(b)
}

func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	re.compile()
	return re.regexp.FindAll(b, n)
}

func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	re.compile()
	return re.regexp.FindAllIndex(b, n)
}

func (re *Regexp) FindAllString(s string, n int) []string {
	re.compile()
	return re.regexp.FindAllString(s, n)
}

func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	re.compile()
	return re.regexp.FindAllStringIndex(s, n)
}

func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	re.compile()
	return re.regexp.FindAllStringSubmatch(s, n)
}

func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	re.compile()
	return re.regexp.FindAllStringSubmatchIndex(s, n)
}

func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	re.compile()
	return re.regexp.FindAllSubmatch(b, n)
}

func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	re.compile()
	return re.regexp.FindAllSubmatchIndex(b, n)
}

func (re *Regexp) FindIndex(b []byte) (loc []int) {
	re.compile()
	return re.regexp.FindIndex(b)
}

func (re *Regexp) FindReaderIndex(r io.RuneReader) (loc []int) {
	re.compile()
	return re.regexp.FindReaderIndex(r)
}

func (re *Regexp) FindReaderSubmatchIndex(r io.RuneReader) []int {
	re.compile()
	return re.regexp.FindReaderSubmatchIndex(r)
}

func (re *Regexp) FindString(s string) string {
	re.compile()
	return re.regexp.FindString(s)
}

func (re *Regexp) FindStringIndex(s string) (loc []int) {
	re.compile()
	return re.regexp.FindStringIndex(s)
}

func (re *Regexp) FindStringSubmatch(s string) []string {
	re.compile()
	return re.regexp.FindStringSubmatch(s)
}

func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	re.compile()
	return re.regexp.FindStringSubmatchIndex(s)
}

func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	re.compile()
	return re.regexp.FindSubmatch(b)
}

func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	re.compile()
	return re.regexp.FindSubmatchIndex(b)
}

func (re *Regexp) LiteralPrefix() (prefix string, complete bool) {
	re.compile()
	return re.regexp.LiteralPrefix()
}

func (re *Regexp) Longest() {
	re.compile()
	re.regexp.Longest()
}

func (re *Regexp) Match(b []byte) bool {
	re.compile()
	return re.regexp.Match(b)
}

func (re *Regexp) MatchReader(r io.RuneReader) bool {
	re.compile()
	return re.regexp.MatchReader(r)
}
func (re *Regexp) MatchString(s string) bool {
	re.compile()
	return re.regexp.MatchString(s)
}

func (re *Regexp) NumSubexp() int {
	re.compile()
	return re.regexp.NumSubexp()
}

func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	re.compile()
	return re.regexp.ReplaceAll(src, repl)
}

func (re *Regexp) ReplaceAllFunc(src []byte, repl func([]byte) []byte) []byte {
	re.compile()
	return re.regexp.ReplaceAllFunc(src, repl)
}

func (re *Regexp) ReplaceAllLiteral(src, repl []byte) []byte {
	re.compile()
	return re.regexp.ReplaceAllLiteral(src, repl)
}

func (re *Regexp) ReplaceAllLiteralString(src, repl string) string {
	re.compile()
	return re.regexp.ReplaceAllLiteralString(src, repl)
}

func (re *Regexp) ReplaceAllString(src, repl string) string {
	re.compile()
	return re.regexp.ReplaceAllString(src, repl)
}

func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	re.compile()
	return re.regexp.ReplaceAllStringFunc(src, repl)
}

func (re *Regexp) Split(s string, n int) []string {
	re.compile()
	return re.regexp.Split(s, n)
}

func (re *Regexp) String() string {
	re.compile()
	return re.regexp.String()
}

func (re *Regexp) SubexpIndex(name string) int {
	re.compile()
	return re.regexp.SubexpIndex(name)
}

func (re *Regexp) SubexpNames() []string {
	re.compile()
	return re.regexp.SubexpNames()
}
