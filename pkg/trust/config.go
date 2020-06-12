package trust

// Policy describes a basic trust policy configuration
type Policy struct {
	Name           string   `json:"name"`
	RepoName       string   `json:"repo_name,omitempty"`
	Keys           []string `json:"keys,omitempty"`
	SignatureStore string   `json:"sigstore"`
	Transport      string   `json:"transport"`
	Type           string   `json:"type"`
	GPGId          string   `json:"gpg_id,omitempty"`
}
