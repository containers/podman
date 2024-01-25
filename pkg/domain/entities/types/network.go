package types

// NetworkPruneReport containers the name of network and an error
// associated in its pruning (removal)
// swagger:model NetworkPruneReport
type NetworkPruneReport struct {
	Name  string
	Error error
}
