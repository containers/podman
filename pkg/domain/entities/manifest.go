package entities

type ManifestCreateOptions struct {
	All bool `schema:"all"`
}

type ManifestAddOptions struct {
	All        bool     `json:"all" schema:"all"`
	Annotation []string `json:"annotation" schema:"annotation"`
	Arch       string   `json:"arch" schema:"arch"`
	Features   []string `json:"features" schema:"features"`
	Images     []string `json:"images" schema:"images"`
	OS         string   `json:"os" schema:"os"`
	OSVersion  string   `json:"os_version" schema:"os_version"`
	Variant    string   `json:"variant" schema:"variant"`
}
