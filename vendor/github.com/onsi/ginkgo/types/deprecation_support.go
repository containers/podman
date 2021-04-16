package types

import (
	"github.com/onsi/ginkgo/formatter"
)

type Deprecation struct {
	Message string
	DocLink string
}

type deprecations struct{}

var Deprecations = deprecations{}

func (d deprecations) CustomReporter() Deprecation {
	return Deprecation{
		Message: "You are using a custom reporter.  Support for custom reporters will likely be removed in V2.  Most users were using them to generate junit or teamcity reports and this functionality will be merged into the core reporter.  In addition, Ginkgo 2.0 will support emitting a JSON-formatted report that users can then manipulate to generate custom reports.\n\n{{red}}{{bold}}If this change will be impactful to you please leave a comment on {{cyan}}{{underline}}https://github.com/onsi/ginkgo/issues/711{{/}}",
		DocLink: "removed-custom-reporters",
	}
}

func (d deprecations) V1Reporter() Deprecation {
	return Deprecation{
		Message: "You are using a V1 Ginkgo Reporter.  Please update your custom reporter to the new V2 Reporter interface.",
		DocLink: "changed-reporter-interface",
	}
}

func (d deprecations) Async() Deprecation {
	return Deprecation{
		Message: "You are passing a Done channel to a test node to test asynchronous behavior.  This is deprecated in Ginkgo V2.  Your test will run synchronously and the timeout will be ignored.",
		DocLink: "removed-async-testing",
	}
}

func (d deprecations) Measure() Deprecation {
	return Deprecation{
		Message: "Measure is deprecated in Ginkgo V2",
		DocLink: "removed-measure",
	}
}

func (d deprecations) Convert() Deprecation {
	return Deprecation{
		Message: "The convert command is deprecated in Ginkgo V2",
		DocLink: "removed-ginkgo-convert",
	}
}

func (d deprecations) Blur() Deprecation {
	return Deprecation{
		Message: "The blur command is deprecated in Ginkgo V2.  Use 'ginkgo unfocus' instead.",
	}
}

type DeprecationTracker struct {
	deprecations map[Deprecation][]CodeLocation
}

func NewDeprecationTracker() *DeprecationTracker {
	return &DeprecationTracker{
		deprecations: map[Deprecation][]CodeLocation{},
	}
}

func (d *DeprecationTracker) TrackDeprecation(deprecation Deprecation, cl ...CodeLocation) {
	if len(cl) == 1 {
		d.deprecations[deprecation] = append(d.deprecations[deprecation], cl[0])
	} else {
		d.deprecations[deprecation] = []CodeLocation{}
	}
}

func (d *DeprecationTracker) DidTrackDeprecations() bool {
	return len(d.deprecations) > 0
}

func (d *DeprecationTracker) DeprecationsReport() string {
	out := formatter.F("{{light-yellow}}You're using deprecated Ginkgo functionality:{{/}}\n")
	out += formatter.F("{{light-yellow}}============================================={{/}}\n")
	out += formatter.F("Ginkgo 2.0 is under active development and will introduce (a small number of) breaking changes.\n")
	out += formatter.F("To learn more, view the migration guide at {{cyan}}{{underline}}https://github.com/onsi/ginkgo/blob/v2/docs/MIGRATING_TO_V2.md{{/}}\n")
	out += formatter.F("To comment, chime in at {{cyan}}{{underline}}https://github.com/onsi/ginkgo/issues/711{{/}}\n\n")

	for deprecation, locations := range d.deprecations {
		out += formatter.Fi(1, "{{yellow}}"+deprecation.Message+"{{/}}\n")
		if deprecation.DocLink != "" {
			out += formatter.Fi(1, "{{bold}}Learn more at:{{/}} {{cyan}}{{underline}}https://github.com/onsi/ginkgo/blob/v2/docs/MIGRATING_TO_V2.md#%s{{/}}\n", deprecation.DocLink)
		}
		for _, location := range locations {
			out += formatter.Fi(2, "{{gray}}%s{{/}}\n", location)
		}
	}
	return out
}
