package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIServer holds configuration (like serving certificates, client CA and CORS domains)
// shared by all API servers in the system, among them especially kube-apiserver
// and openshift-apiserver. The canonical name of an instance is 'cluster'.
type APIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required
	// +required
	Spec APIServerSpec `json:"spec"`
	// +optional
	Status APIServerStatus `json:"status"`
}

type APIServerSpec struct {
	// servingCert is the TLS cert info for serving secure traffic. If not specified, operator managed certificates
	// will be used for serving secure traffic.
	// +optional
	ServingCerts APIServerServingCerts `json:"servingCerts"`
	// clientCA references a ConfigMap containing a certificate bundle for the signers that will be recognized for
	// incoming client certificates in addition to the operator managed signers. If this is empty, then only operator managed signers are valid.
	// You usually only have to set this if you have your own PKI you wish to honor client certificates from.
	// The ConfigMap must exist in the openshift-config namespace and contain the following required fields:
	// - ConfigMap.Data["ca-bundle.crt"] - CA bundle.
	// +optional
	ClientCA ConfigMapNameReference `json:"clientCA"`
	// additionalCORSAllowedOrigins lists additional, user-defined regular expressions describing hosts for which the
	// API server allows access using the CORS headers. This may be needed to access the API and the integrated OAuth
	// server from JavaScript applications.
	// The values are regular expressions that correspond to the Golang regular expression language.
	// +optional
	AdditionalCORSAllowedOrigins []string `json:"additionalCORSAllowedOrigins,omitempty"`
}

type APIServerServingCerts struct {
	// namedCertificates references secrets containing the TLS cert info for serving secure traffic to specific hostnames.
	// If no named certificates are provided, or no named certificates match the server name as understood by a client,
	// the defaultServingCertificate will be used.
	// +optional
	NamedCertificates []APIServerNamedServingCert `json:"namedCertificates,omitempty"`
}

// APIServerNamedServingCert maps a server DNS name, as understood by a client, to a certificate.
type APIServerNamedServingCert struct {
	// names is a optional list of explicit DNS names (leading wildcards allowed) that should use this certificate to
	// serve secure traffic. If no names are provided, the implicit names will be extracted from the certificates.
	// Exact names trump over wildcard names. Explicit names defined here trump over extracted implicit names.
	// +optional
	Names []string `json:"names,omitempty"`
	// servingCertificate references a kubernetes.io/tls type secret containing the TLS cert info for serving secure traffic.
	// The secret must exist in the openshift-config namespace and contain the following required fields:
	// - Secret.Data["tls.key"] - TLS private key.
	// - Secret.Data["tls.crt"] - TLS certificate.
	ServingCertificate SecretNameReference `json:"servingCertificate"`
}

type APIServerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type APIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []APIServer `json:"items"`
}
