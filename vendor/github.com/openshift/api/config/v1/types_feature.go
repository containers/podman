package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Feature holds cluster-wide information about feature gates.  The canonical name is `cluster`
type FeatureGate struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec FeatureGateSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status FeatureGateStatus `json:"status"`
}

type FeatureSet string

var (
	// Default feature set that allows upgrades.
	Default FeatureSet = ""

	// TechPreviewNoUpgrade turns on tech preview features that are not part of the normal supported platform. Turning
	// this feature set on CANNOT BE UNDONE and PREVENTS UPGRADES.
	TechPreviewNoUpgrade FeatureSet = "TechPreviewNoUpgrade"

	// CustomNoUpgrade allows the enabling or disabling of any feature. Turning this feature set on IS NOT SUPPORTED, CANNOT BE UNDONE, and PREVENTS UPGRADES.
	// Because of its nature, this setting cannot be validated.  If you have any typos or accidentally apply invalid combinations
	// your cluster may fail in an unrecoverable way.
	CustomNoUpgrade FeatureSet = "CustomNoUpgrade"
)

type FeatureGateSpec struct {
	FeatureGateSelection `json:",inline"`
}

// +union
type FeatureGateSelection struct {
	// featureSet changes the list of features in the cluster.  The default is empty.  Be very careful adjusting this setting.
	// Turning on or off features may cause irreversible changes in your cluster which cannot be undone.
	// +unionDiscriminator
	// +optional
	FeatureSet FeatureSet `json:"featureSet,omitempty"`

	// customNoUpgrade allows the enabling or disabling of any feature. Turning this feature set on IS NOT SUPPORTED, CANNOT BE UNDONE, and PREVENTS UPGRADES.
	// Because of its nature, this setting cannot be validated.  If you have any typos or accidentally apply invalid combinations
	// your cluster may fail in an unrecoverable way.  featureSet must equal "CustomNoUpgrade" must be set to use this field.
	// +optional
	// +nullable
	CustomNoUpgrade *CustomFeatureGates `json:"customNoUpgrade,omitempty"`
}

type CustomFeatureGates struct {
	// enabled is a list of all feature gates that you want to force on
	// +optional
	Enabled []string `json:"enabled,omitempty"`
	// disabled is a list of all feature gates that you want to force off
	// +optional
	Disabled []string `json:"disabled,omitempty"`
}

type FeatureGateStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FeatureGateList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []FeatureGate `json:"items"`
}

type FeatureGateEnabledDisabled struct {
	Enabled  []string
	Disabled []string
}

// FeatureSets Contains a map of Feature names to Enabled/Disabled Feature.
//
// NOTE: The caller needs to make sure to check for the existence of the value
// using golang's existence field. A possible scenario is an upgrade where new
// FeatureSets are added and a controller has not been upgraded with a newer
// version of this file. In this upgrade scenario the map could return nil.
//
// example:
//   if featureSet, ok := FeatureSets["SomeNewFeature"]; ok { }
//
// If you put an item in either of these lists, put your area and name on it so we can find owners.
var FeatureSets = map[FeatureSet]*FeatureGateEnabledDisabled{
	Default: {
		Enabled: []string{
			"ExperimentalCriticalPodAnnotation", // sig-pod, sjenning
			"RotateKubeletServerCertificate",    // sig-pod, sjenning
			"SupportPodPidsLimit",               // sig-pod, sjenning
		},
		Disabled: []string{
			"LocalStorageCapacityIsolation", // sig-pod, sjenning
		},
	},
	TechPreviewNoUpgrade: {
		Enabled: []string{
			"ExperimentalCriticalPodAnnotation", // sig-pod, sjenning
			"RotateKubeletServerCertificate",    // sig-pod, sjenning
			"SupportPodPidsLimit",               // sig-pod, sjenning
		},
		Disabled: []string{
			"LocalStorageCapacityIsolation", // sig-pod, sjenning
		},
	},
}
