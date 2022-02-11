package rhcos

// Extensions is data specific to Red Hat Enterprise Linux CoreOS
type Extensions struct {
	AzureDisk *AzureDisk `json:"azure-disk,omitempty"`
}

// AzureDisk represents an Azure cloud image.
type AzureDisk struct {
	// URL to an image already stored in Azure infrastructure
	// that can be copied into an image gallery.  Avoid creating VMs directly
	// from this URL as that may lead to performance limitations.
	URL string `json:"url,omitempty"`
}
