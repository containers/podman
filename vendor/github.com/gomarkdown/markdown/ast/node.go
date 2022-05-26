package ast

// ListType contains bitwise or'ed flags for list and list item objects.
type ListType int

// These are the possible flag values for the ListItem renderer.
// Multiple flag values may be ORed together.
// These are mostly of interest if you are writing a new output format.
const (
	ListTypeOrdered ListType = 1 << iota
	ListTypeDefinition
	ListTypeTerm

	ListItemContainsBlock
	ListItemBeginningOfList // TODO: figure out if this is of any use now
	ListItemEndOfList
)

// CellAlignFlags holds a type of alignment in a table cell.
type CellAlignFlags int

// These are the possible flag values for the table cell renderer.
// Only a single one of these values will be used; they are not ORed together.
// These are mostly of interest if you are writing a new output format.
const (
	TableAlignmentLeft CellAlignFlags = 1 << iota
	TableAlignmentRight
	TableAlignmentCenter = (TableAlignmentLeft | TableAlignmentRight)
)

func (a CellAlignFlags) String() string {
	switch a {
	case TableAlignmentLeft:
		return "left"
	case TableAlignmentRight:
		return "right"
	case TableAlignmentCenter:
		return "center"
	default:
		return ""
	}
}

// DocumentMatters holds the type of a {front,main,back}matter in the document
type DocumentMatters int

// These are all possible Document divisions.
const (
	DocumentMatterNone DocumentMatters = iota
	DocumentMatterFront
	DocumentMatterMain
	DocumentMatterBack
)

// CitationTypes holds the type of a citation, informative, normative or suppressed
type CitationTypes int

const (
	CitationTypeNone CitationTypes = iota
	CitationTypeSuppressed
	CitationTypeInformative
	CitationTypeNormative
)

// Node defines an ast node
type Node interface {
	AsContainer() *Container
	AsLeaf() *Leaf
	GetParent() Node
	SetParent(newParent Node)
	GetChildren() []Node
	SetChildren(newChildren []Node)
}

// Container is a type of node that can contain children
type Container struct {
	Parent   Node
	Children []Node

	Literal []byte // Text contents of the leaf nodes
	Content []byte // Markdown content of the block nodes

	*Attribute // Block level attribute
}

// AsContainer returns itself as *Container
func (c *Container) AsContainer() *Container {
	return c
}

// AsLeaf returns nil
func (c *Container) AsLeaf() *Leaf {
	return nil
}

// GetParent returns parent node
func (c *Container) GetParent() Node {
	return c.Parent
}

// SetParent sets the parent node
func (c *Container) SetParent(newParent Node) {
	c.Parent = newParent
}

// GetChildren returns children nodes
func (c *Container) GetChildren() []Node {
	return c.Children
}

// SetChildren sets children node
func (c *Container) SetChildren(newChildren []Node) {
	c.Children = newChildren
}

// Leaf is a type of node that cannot have children
type Leaf struct {
	Parent Node

	Literal []byte // Text contents of the leaf nodes
	Content []byte // Markdown content of the block nodes

	*Attribute // Block level attribute
}

// AsContainer returns nil
func (l *Leaf) AsContainer() *Container {
	return nil
}

// AsLeaf returns itself as *Leaf
func (l *Leaf) AsLeaf() *Leaf {
	return l
}

// GetParent returns parent node
func (l *Leaf) GetParent() Node {
	return l.Parent
}

// SetParent sets the parent nodd
func (l *Leaf) SetParent(newParent Node) {
	l.Parent = newParent
}

// GetChildren returns nil because Leaf cannot have children
func (l *Leaf) GetChildren() []Node {
	return nil
}

// SetChildren will panic becuase Leaf cannot have children
func (l *Leaf) SetChildren(newChildren []Node) {
	panic("leaf node cannot have children")
}

// Document represents markdown document node, a root of ast
type Document struct {
	Container
}

// DocumentMatter represents markdown node that signals a document
// division: frontmatter, mainmatter or backmatter.
type DocumentMatter struct {
	Container

	Matter DocumentMatters
}

// BlockQuote represents markdown block quote node
type BlockQuote struct {
	Container
}

// Aside represents an markdown aside node.
type Aside struct {
	Container
}

// List represents markdown list node
type List struct {
	Container

	ListFlags       ListType
	Tight           bool   // Skip <p>s around list item data if true
	BulletChar      byte   // '*', '+' or '-' in bullet lists
	Delimiter       byte   // '.' or ')' after the number in ordered lists
	Start           int    // for ordered lists this indicates the starting number if > 0
	RefLink         []byte // If not nil, turns this list item into a footnote item and triggers different rendering
	IsFootnotesList bool   // This is a list of footnotes
}

// ListItem represents markdown list item node
type ListItem struct {
	Container

	ListFlags       ListType
	Tight           bool   // Skip <p>s around list item data if true
	BulletChar      byte   // '*', '+' or '-' in bullet lists
	Delimiter       byte   // '.' or ')' after the number in ordered lists
	RefLink         []byte // If not nil, turns this list item into a footnote item and triggers different rendering
	IsFootnotesList bool   // This is a list of footnotes
}

// Paragraph represents markdown paragraph node
type Paragraph struct {
	Container
}

// Math represents markdown MathAjax inline node
type Math struct {
	Leaf
}

// MathBlock represents markdown MathAjax block node
type MathBlock struct {
	Container
}

// Heading represents markdown heading node
type Heading struct {
	Container

	Level        int    // This holds the heading level number
	HeadingID    string // This might hold heading ID, if present
	IsTitleblock bool   // Specifies whether it's a title block
	IsSpecial    bool   // We are a special heading (starts with .#)
}

// HorizontalRule represents markdown horizontal rule node
type HorizontalRule struct {
	Leaf
}

// Emph represents markdown emphasis node
type Emph struct {
	Container
}

// Strong represents markdown strong node
type Strong struct {
	Container
}

// Del represents markdown del node
type Del struct {
	Container
}

// Link represents markdown link node
type Link struct {
	Container

	Destination          []byte   // Destination is what goes into a href
	Title                []byte   // Title is the tooltip thing that goes in a title attribute
	NoteID               int      // NoteID contains a serial number of a footnote, zero if it's not a footnote
	Footnote             Node     // If it's a footnote, this is a direct link to the footnote Node. Otherwise nil.
	DeferredID           []byte   // If a deferred link this holds the original ID.
	AdditionalAttributes []string // Defines additional attributes to use during rendering.
}

// CrossReference is a reference node.
type CrossReference struct {
	Container

	Destination []byte // Destination is where the reference points to
}

// Citation is a citation node.
type Citation struct {
	Leaf

	Destination [][]byte        // Destination is where the citation points to. Multiple ones are allowed.
	Type        []CitationTypes // 1:1 mapping of destination and citation type
	Suffix      [][]byte        // Potential citation suffix, i.e. [@!RFC1035, p. 144]
}

// Image represents markdown image node
type Image struct {
	Container

	Destination []byte // Destination is what goes into a href
	Title       []byte // Title is the tooltip thing that goes in a title attribute
}

// Text represents markdown text node
type Text struct {
	Leaf
}

// HTMLBlock represents markdown html node
type HTMLBlock struct {
	Leaf
}

// CodeBlock represents markdown code block node
type CodeBlock struct {
	Leaf

	IsFenced    bool   // Specifies whether it's a fenced code block or an indented one
	Info        []byte // This holds the info string
	FenceChar   byte
	FenceLength int
	FenceOffset int
}

// Softbreak represents markdown softbreak node
// Note: not used currently
type Softbreak struct {
	Leaf
}

// Hardbreak represents markdown hard break node
type Hardbreak struct {
	Leaf
}

// NonBlockingSpace represents markdown non-blocking space node
type NonBlockingSpace struct {
	Leaf
}

// Code represents markdown code node
type Code struct {
	Leaf
}

// HTMLSpan represents markdown html span node
type HTMLSpan struct {
	Leaf
}

// Table represents markdown table node
type Table struct {
	Container
}

// TableCell represents markdown table cell node
type TableCell struct {
	Container

	IsHeader bool           // This tells if it's under the header row
	Align    CellAlignFlags // This holds the value for align attribute
	ColSpan  int            // How many columns to span
}

// TableHeader represents markdown table head node
type TableHeader struct {
	Container
}

// TableBody represents markdown table body node
type TableBody struct {
	Container
}

// TableRow represents markdown table row node
type TableRow struct {
	Container
}

// TableFooter represents markdown table foot node
type TableFooter struct {
	Container
}

// Caption represents a figure, code or quote caption
type Caption struct {
	Container
}

// CaptionFigure is a node (blockquote or codeblock) that has a caption
type CaptionFigure struct {
	Container

	HeadingID string // This might hold heading ID, if present
}

// Callout is a node that can exist both in text (where it is an actual node) and in a code block.
type Callout struct {
	Leaf

	ID []byte // number of this callout
}

// Index is a node that contains an Index item and an optional, subitem.
type Index struct {
	Leaf

	Primary bool
	Item    []byte
	Subitem []byte
	ID      string // ID of the index
}

// Subscript is a subscript node
type Subscript struct {
	Leaf
}

// Subscript is a superscript node
type Superscript struct {
	Leaf
}

// Footnotes is a node that contains all footnotes
type Footnotes struct {
	Container
}

func removeNodeFromArray(a []Node, node Node) []Node {
	n := len(a)
	for i := 0; i < n; i++ {
		if a[i] == node {
			return append(a[:i], a[i+1:]...)
		}
	}
	return nil
}

// AppendChild appends child to children of parent
// It panics if either node is nil.
func AppendChild(parent Node, child Node) {
	RemoveFromTree(child)
	child.SetParent(parent)
	newChildren := append(parent.GetChildren(), child)
	parent.SetChildren(newChildren)
}

// RemoveFromTree removes this node from tree
func RemoveFromTree(n Node) {
	if n.GetParent() == nil {
		return
	}
	// important: don't clear n.Children if n has no parent
	// we're called from AppendChild and that might happen on a node
	// that accumulated Children but hasn't been inserted into the tree
	n.SetChildren(nil)
	p := n.GetParent()
	newChildren := removeNodeFromArray(p.GetChildren(), n)
	if newChildren != nil {
		p.SetChildren(newChildren)
	}
}

// GetLastChild returns last child of node n
// It's implemented as stand-alone function to keep Node interface small
func GetLastChild(n Node) Node {
	a := n.GetChildren()
	if len(a) > 0 {
		return a[len(a)-1]
	}
	return nil
}

// GetFirstChild returns first child of node n
// It's implemented as stand-alone function to keep Node interface small
func GetFirstChild(n Node) Node {
	a := n.GetChildren()
	if len(a) > 0 {
		return a[0]
	}
	return nil
}

// GetNextNode returns next sibling of node n (node after n)
// We can't make it part of Container or Leaf because we loose Node identity
func GetNextNode(n Node) Node {
	parent := n.GetParent()
	if parent == nil {
		return nil
	}
	a := parent.GetChildren()
	len := len(a) - 1
	for i := 0; i < len; i++ {
		if a[i] == n {
			return a[i+1]
		}
	}
	return nil
}

// GetPrevNode returns previous sibling of node n (node before n)
// We can't make it part of Container or Leaf because we loose Node identity
func GetPrevNode(n Node) Node {
	parent := n.GetParent()
	if parent == nil {
		return nil
	}
	a := parent.GetChildren()
	len := len(a)
	for i := 1; i < len; i++ {
		if a[i] == n {
			return a[i-1]
		}
	}
	return nil
}

// WalkStatus allows NodeVisitor to have some control over the tree traversal.
// It is returned from NodeVisitor and different values allow Node.Walk to
// decide which node to go to next.
type WalkStatus int

const (
	// GoToNext is the default traversal of every node.
	GoToNext WalkStatus = iota
	// SkipChildren tells walker to skip all children of current node.
	SkipChildren
	// Terminate tells walker to terminate the traversal.
	Terminate
)

// NodeVisitor is a callback to be called when traversing the syntax tree.
// Called twice for every node: once with entering=true when the branch is
// first visited, then with entering=false after all the children are done.
type NodeVisitor interface {
	Visit(node Node, entering bool) WalkStatus
}

// NodeVisitorFunc casts a function to match NodeVisitor interface
type NodeVisitorFunc func(node Node, entering bool) WalkStatus

// Walk traverses tree recursively
func Walk(n Node, visitor NodeVisitor) WalkStatus {
	isContainer := n.AsContainer() != nil
	status := visitor.Visit(n, true) // entering
	if status == Terminate {
		// even if terminating, close container node
		if isContainer {
			visitor.Visit(n, false)
		}
		return status
	}
	if isContainer && status != SkipChildren {
		children := n.GetChildren()
		for _, n := range children {
			status = Walk(n, visitor)
			if status == Terminate {
				return status
			}
		}
	}
	if isContainer {
		status = visitor.Visit(n, false) // exiting
		if status == Terminate {
			return status
		}
	}
	return GoToNext
}

// Visit calls visitor function
func (f NodeVisitorFunc) Visit(node Node, entering bool) WalkStatus {
	return f(node, entering)
}

// WalkFunc is like Walk but accepts just a callback function
func WalkFunc(n Node, f NodeVisitorFunc) {
	visitor := NodeVisitorFunc(f)
	Walk(n, visitor)
}
