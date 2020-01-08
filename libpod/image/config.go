package image

// ImageDeleteResponse is the response for removing an image from storage and containers
// what was untagged vs actually removed
type ImageDeleteResponse struct { //nolint
	Untagged []string `json:"untagged"`
	Deleted  string   `json:"deleted"`
}
