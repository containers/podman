package trust

// Policy describes a basic trust policy configuration
type Policy struct {
	Transport      string   `json:"transport"`
	Name           string   `json:"name,omitempty"`
	RepoName       string   `json:"repo_name,omitempty"`
	Keys           []string `json:"keys,omitempty"`
	SignatureStore string   `json:"sigstore,omitempty"`
	Type           string   `json:"type"`
	GPGId          string   `json:"gpg_id,omitempty"`
}
