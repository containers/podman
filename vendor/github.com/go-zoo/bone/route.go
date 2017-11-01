/********************************
*** Multiplexer for Go        ***
*** Bone is under MIT license ***
*** Code by CodingFerret      ***
*** github.com/go-zoo         ***
*********************************/

package bone

import (
	"net/http"
	"regexp"
	"strings"
)

const (
	//PARAM value store in Atts if the route have parameters
	PARAM = 2
	//SUB value store in Atts if the route is a sub router
	SUB = 4
	//WC value store in Atts if the route have wildcard
	WC = 8
	//REGEX value store in Atts if the route contains regex
	REGEX = 16
)

// Route content the required information for a valid route
// Path: is the Route URL
// Size: is the length of the path
// Token: is the value of each part of the path, split by /
// Pattern: is content information about the route, if it's have a route variable
// handler: is the handler who handle this route
// Method: define HTTP method on the route
type Route struct {
	Path    string
	Method  string
	Size    int
	Atts    int
	wildPos int
	Token   Token
	Pattern map[int]string
	Compile map[int]*regexp.Regexp
	Tag     map[int]string
	Handler http.Handler
}

// Token content all value of a spliting route path
// Tokens: string value of each token
// size: number of token
type Token struct {
	raw    []int
	Tokens []string
	Size   int
}

// NewRoute return a pointer to a Route instance and call save() on it
func NewRoute(url string, h http.Handler) *Route {
	r := &Route{Path: url, Handler: h}
	r.save()
	return r
}

// Save, set automatically the the Route.Size and Route.Pattern value
func (r *Route) save() {
	r.Size = len(r.Path)
	r.Token.Tokens = strings.Split(r.Path, "/")
	for i, s := range r.Token.Tokens {
		if len(s) >= 1 {
			switch s[:1] {
			case ":":
				if r.Pattern == nil {
					r.Pattern = make(map[int]string)
				}
				r.Pattern[i] = s[1:]
				r.Atts |= PARAM
			case "#":
				if r.Compile == nil {
					r.Compile = make(map[int]*regexp.Regexp)
					r.Tag = make(map[int]string)
				}
				tmp := strings.Split(s, "^")
				r.Tag[i] = tmp[0][1:]
				r.Compile[i] = regexp.MustCompile("^" + tmp[1][:len(tmp[1])-1])
				r.Atts |= REGEX
			case "*":
				r.wildPos = i
				r.Atts |= WC
			default:
				r.Token.raw = append(r.Token.raw, i)
			}
		}
		r.Token.Size++
	}
}

// Match check if the request match the route Pattern
func (r *Route) Match(req *http.Request) bool {
	ok, _ := r.matchAndParse(req)
	return ok
}

// matchAndParse check if the request matches the route Pattern and returns a map of the parsed
// variables if it matches
func (r *Route) matchAndParse(req *http.Request) (bool, map[string]string) {
	ss := strings.Split(req.URL.EscapedPath(), "/")
	if r.matchRawTokens(&ss) {
		if len(ss) == r.Token.Size || r.Atts&WC != 0 {
			totalSize := len(r.Pattern)
			if r.Atts&REGEX != 0 {
				totalSize += len(r.Compile)
			}

			vars := make(map[string]string, totalSize)
			for k, v := range r.Pattern {
				vars[v] = ss[k]
			}

			if r.Atts&REGEX != 0 {
				for k, v := range r.Compile {
					if !v.MatchString(ss[k]) {
						return false, nil
					}
					vars[r.Tag[k]] = ss[k]
				}
			}

			return true, vars
		}
	}

	return false, nil
}

func (r *Route) parse(rw http.ResponseWriter, req *http.Request) bool {
	if r.Atts != 0 {
		if r.Atts&SUB != 0 {
			if len(req.URL.Path) >= r.Size {
				if req.URL.Path[:r.Size] == r.Path {
					req.URL.Path = req.URL.Path[r.Size:]
					r.Handler.ServeHTTP(rw, req)
					return true
				}
			}
		}

		if ok, vars := r.matchAndParse(req); ok {
			r.serveMatchedRequest(rw, req, vars)
			return true
		}
	}
	if req.URL.Path == r.Path {
		r.Handler.ServeHTTP(rw, req)
		return true
	}
	return false
}

func (r *Route) matchRawTokens(ss *[]string) bool {
	if len(*ss) >= r.Token.Size {
		for i, v := range r.Token.raw {
			if (*ss)[v] != r.Token.Tokens[v] {
				if r.Atts&WC != 0 && r.wildPos == i {
					return true
				}
				return false
			}
		}
		return true
	}
	return false
}

func (r *Route) exists(rw http.ResponseWriter, req *http.Request) bool {
	if r.Atts != 0 {
		if r.Atts&SUB != 0 {
			if len(req.URL.Path) >= r.Size {
				if req.URL.Path[:r.Size] == r.Path {
					return true
				}
			}
		}

		if ok, _ := r.matchAndParse(req); ok {
			return true
		}
	}
	if req.URL.Path == r.Path {
		return true
	}
	return false
}

// Get set the route method to Get
func (r *Route) Get() *Route {
	r.Method = "GET"
	return r
}

// Post set the route method to Post
func (r *Route) Post() *Route {
	r.Method = "POST"
	return r
}

// Put set the route method to Put
func (r *Route) Put() *Route {
	r.Method = "PUT"
	return r
}

// Delete set the route method to Delete
func (r *Route) Delete() *Route {
	r.Method = "DELETE"
	return r
}

// Head set the route method to Head
func (r *Route) Head() *Route {
	r.Method = "HEAD"
	return r
}

// Patch set the route method to Patch
func (r *Route) Patch() *Route {
	r.Method = "PATCH"
	return r
}

// Options set the route method to Options
func (r *Route) Options() *Route {
	r.Method = "OPTIONS"
	return r
}

func (r *Route) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if r.Method != "" {
		if req.Method == r.Method {
			r.Handler.ServeHTTP(rw, req)
			return
		}
		http.NotFound(rw, req)
		return
	}
	r.Handler.ServeHTTP(rw, req)
}
