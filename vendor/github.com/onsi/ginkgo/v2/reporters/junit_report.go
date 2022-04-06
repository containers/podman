/*

JUnit XML Reporter for Ginkgo

For usage instructions: http://onsi.github.io/ginkgo/#generating_junit_xml_output

The schema used for the generated JUnit xml file was adapted from https://llg.cubic.org/docs/junit/

*/

package reporters

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2/config"
	"github.com/onsi/ginkgo/v2/types"
)

type JUnitTestSuites struct {
	XMLName xml.Name `xml:"testsuites"`
	// Tests maps onto the total number of specs in all test suites (this includes any suite nodes such as BeforeSuite)
	Tests int `xml:"tests,attr"`
	// Disabled maps onto specs that are pending and/or skipped
	Disabled int `xml:"disabled,attr"`
	// Errors maps onto specs that panicked or were interrupted
	Errors int `xml:"errors,attr"`
	// Failures maps onto specs that failed
	Failures int `xml:"failures,attr"`
	// Time is the time in seconds to execute all test suites
	Time float64 `xml:"time,attr"`

	//The set of all test suites
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	// Name maps onto the description of the test suite - maps onto Report.SuiteDescription
	Name string `xml:"name,attr"`
	// Package maps onto the absolute path to the test suite - maps onto Report.SuitePath
	Package string `xml:"package,attr"`
	// Tests maps onto the total number of specs in the test suite (this includes any suite nodes such as BeforeSuite)
	Tests int `xml:"tests,attr"`
	// Disabled maps onto specs that are pending
	Disabled int `xml:"disabled,attr"`
	// Skiped maps onto specs that are skipped
	Skipped int `xml:"skipped,attr"`
	// Errors maps onto specs that panicked or were interrupted
	Errors int `xml:"errors,attr"`
	// Failures maps onto specs that failed
	Failures int `xml:"failures,attr"`
	// Time is the time in seconds to execute all the test suite - maps onto Report.RunTime
	Time float64 `xml:"time,attr"`
	// Timestamp is the ISO 8601 formatted start-time of the suite - maps onto Report.StartTime
	Timestamp string `xml:"timestamp,attr"`

	//Properties captures the information stored in the rest of the Report type (including SuiteConfig) as key-value pairs
	Properties JUnitProperties `xml:"properties"`

	//TestCases capture the individual specs
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitProperties struct {
	Properties []JUnitProperty `xml:"property"`
}

func (jup JUnitProperties) WithName(name string) string {
	for _, property := range jup.Properties {
		if property.Name == name {
			return property.Value
		}
	}
	return ""
}

type JUnitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type JUnitTestCase struct {
	// Name maps onto the full text of the spec - equivalent to "[SpecReport.LeafNodeType] SpecReport.FullText()"
	Name string `xml:"name,attr"`
	// Classname maps onto the name of the test suite - equivalent to Report.SuiteDescription
	Classname string `xml:"classname,attr"`
	// Status maps onto the string representation of SpecReport.State
	Status string `xml:"status,attr"`
	// Time is the time in seconds to execute the spec - maps onto SpecReport.RunTime
	Time float64 `xml:"time,attr"`
	//Skipped is populated with a message if the test was skipped or pending
	Skipped *JUnitSkipped `xml:"skipped,omitempty"`
	//Error is populated if the test panicked or was interrupted
	Error *JUnitError `xml:"error,omitempty"`
	//Failure is populated if the test failed
	Failure *JUnitFailure `xml:"failure,omitempty"`
	//SystemOut maps onto any captured stdout/stderr output - maps onto SpecReport.CapturedStdOutErr
	SystemOut string `xml:"system-out,omitempty"`
	//SystemOut maps onto any captured GinkgoWriter output - maps onto SpecReport.CapturedGinkgoWriterOutput
	SystemErr string `xml:"system-err,omitempty"`
}

type JUnitSkipped struct {
	// Message maps onto "pending" if the test was marked pending, "skipped" if the test was marked skipped, and "skipped - REASON" if the user called Skip(REASON)
	Message string `xml:"message,attr"`
}

type JUnitError struct {
	//Message maps onto the panic/exception thrown - equivalent to SpecReport.Failure.ForwardedPanic - or to "interrupted"
	Message string `xml:"message,attr"`
	//Type is one of "panicked" or "interrupted"
	Type string `xml:"type,attr"`
	//Description maps onto the captured stack trace for a panic, or the failure message for an interrupt which will include the dump of running goroutines
	Description string `xml:",chardata"`
}

type JUnitFailure struct {
	//Message maps onto the failure message - equivalent to SpecReport.Failure.Message
	Message string `xml:"message,attr"`
	//Type is "failed"
	Type string `xml:"type,attr"`
	//Description maps onto the location and stack trace of the failure
	Description string `xml:",chardata"`
}

func GenerateJUnitReport(report types.Report, dst string) error {
	suite := JUnitTestSuite{
		Name:      report.SuiteDescription,
		Package:   report.SuitePath,
		Time:      report.RunTime.Seconds(),
		Timestamp: report.StartTime.Format("2006-01-02T15:04:05"),
		Properties: JUnitProperties{
			Properties: []JUnitProperty{
				{"SuiteSucceeded", fmt.Sprintf("%t", report.SuiteSucceeded)},
				{"SuiteHasProgrammaticFocus", fmt.Sprintf("%t", report.SuiteHasProgrammaticFocus)},
				{"SpecialSuiteFailureReason", strings.Join(report.SpecialSuiteFailureReasons, ",")},
				{"SuiteLabels", fmt.Sprintf("[%s]", strings.Join(report.SuiteLabels, ","))},
				{"RandomSeed", fmt.Sprintf("%d", report.SuiteConfig.RandomSeed)},
				{"RandomizeAllSpecs", fmt.Sprintf("%t", report.SuiteConfig.RandomizeAllSpecs)},
				{"LabelFilter", report.SuiteConfig.LabelFilter},
				{"FocusStrings", strings.Join(report.SuiteConfig.FocusStrings, ",")},
				{"SkipStrings", strings.Join(report.SuiteConfig.SkipStrings, ",")},
				{"FocusFiles", strings.Join(report.SuiteConfig.FocusFiles, ";")},
				{"SkipFiles", strings.Join(report.SuiteConfig.SkipFiles, ";")},
				{"FailOnPending", fmt.Sprintf("%t", report.SuiteConfig.FailOnPending)},
				{"FailFast", fmt.Sprintf("%t", report.SuiteConfig.FailFast)},
				{"FlakeAttempts", fmt.Sprintf("%d", report.SuiteConfig.FlakeAttempts)},
				{"EmitSpecProgress", fmt.Sprintf("%t", report.SuiteConfig.EmitSpecProgress)},
				{"DryRun", fmt.Sprintf("%t", report.SuiteConfig.DryRun)},
				{"ParallelTotal", fmt.Sprintf("%d", report.SuiteConfig.ParallelTotal)},
				{"OutputInterceptorMode", report.SuiteConfig.OutputInterceptorMode},
			},
		},
	}
	for _, spec := range report.SpecReports {
		name := fmt.Sprintf("[%s]", spec.LeafNodeType)
		if spec.FullText() != "" {
			name = name + " " + spec.FullText()
		}
		labels := spec.Labels()
		if len(labels) > 0 {
			name = name + " [" + strings.Join(labels, ", ") + "]"
		}

		test := JUnitTestCase{
			Name:      name,
			Classname: report.SuiteDescription,
			Status:    spec.State.String(),
			Time:      spec.RunTime.Seconds(),
			SystemOut: systemOutForUnstructureReporters(spec),
			SystemErr: spec.CapturedGinkgoWriterOutput,
		}
		suite.Tests += 1

		switch spec.State {
		case types.SpecStateSkipped:
			message := "skipped"
			if spec.Failure.Message != "" {
				message += " - " + spec.Failure.Message
			}
			test.Skipped = &JUnitSkipped{Message: message}
			suite.Skipped += 1
		case types.SpecStatePending:
			test.Skipped = &JUnitSkipped{Message: "pending"}
			suite.Disabled += 1
		case types.SpecStateFailed:
			test.Failure = &JUnitFailure{
				Message:     spec.Failure.Message,
				Type:        "failed",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Failures += 1
		case types.SpecStateInterrupted:
			test.Error = &JUnitError{
				Message:     "interrupted",
				Type:        "interrupted",
				Description: spec.Failure.Message,
			}
			suite.Errors += 1
		case types.SpecStateAborted:
			test.Failure = &JUnitFailure{
				Message:     spec.Failure.Message,
				Type:        "aborted",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Errors += 1
		case types.SpecStatePanicked:
			test.Error = &JUnitError{
				Message:     spec.Failure.ForwardedPanic,
				Type:        "panicked",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Errors += 1
		}

		suite.TestCases = append(suite.TestCases, test)
	}

	junitReport := JUnitTestSuites{
		Tests:      suite.Tests,
		Disabled:   suite.Disabled + suite.Skipped,
		Errors:     suite.Errors,
		Failures:   suite.Failures,
		Time:       suite.Time,
		TestSuites: []JUnitTestSuite{suite},
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	f.WriteString(xml.Header)
	encoder := xml.NewEncoder(f)
	encoder.Indent("  ", "    ")
	encoder.Encode(junitReport)

	return f.Close()
}

func MergeAndCleanupJUnitReports(sources []string, dst string) ([]string, error) {
	messages := []string{}
	mergedReport := JUnitTestSuites{}
	for _, source := range sources {
		report := JUnitTestSuites{}
		f, err := os.Open(source)
		if err != nil {
			messages = append(messages, fmt.Sprintf("Could not open %s:\n%s", source, err.Error()))
			continue
		}
		err = xml.NewDecoder(f).Decode(&report)
		if err != nil {
			messages = append(messages, fmt.Sprintf("Could not decode %s:\n%s", source, err.Error()))
			continue
		}
		os.Remove(source)

		mergedReport.Tests += report.Tests
		mergedReport.Disabled += report.Disabled
		mergedReport.Errors += report.Errors
		mergedReport.Failures += report.Failures
		mergedReport.Time += report.Time
		mergedReport.TestSuites = append(mergedReport.TestSuites, report.TestSuites...)
	}

	f, err := os.Create(dst)
	if err != nil {
		return messages, err
	}
	f.WriteString(xml.Header)
	encoder := xml.NewEncoder(f)
	encoder.Indent("  ", "    ")
	encoder.Encode(mergedReport)

	return messages, f.Close()
}

func systemOutForUnstructureReporters(spec types.SpecReport) string {
	systemOut := spec.CapturedStdOutErr
	if len(spec.ReportEntries) > 0 {
		systemOut += "\nReport Entries:\n"
		for i, entry := range spec.ReportEntries {
			systemOut += fmt.Sprintf("%s\n%s\n%s\n", entry.Name, entry.Location, entry.Time.Format(time.RFC3339Nano))
			if representation := entry.StringRepresentation(); representation != "" {
				systemOut += representation + "\n"
			}
			if i+1 < len(spec.ReportEntries) {
				systemOut += "--\n"
			}
		}
	}
	return systemOut
}

// Deprecated JUnitReporter (so folks can still compile their suites)
type JUnitReporter struct{}

func NewJUnitReporter(_ string) *JUnitReporter                                                  { return &JUnitReporter{} }
func (reporter *JUnitReporter) SuiteWillBegin(_ config.GinkgoConfigType, _ *types.SuiteSummary) {}
func (reporter *JUnitReporter) BeforeSuiteDidRun(_ *types.SetupSummary)                         {}
func (reporter *JUnitReporter) SpecWillRun(_ *types.SpecSummary)                                {}
func (reporter *JUnitReporter) SpecDidComplete(_ *types.SpecSummary)                            {}
func (reporter *JUnitReporter) AfterSuiteDidRun(_ *types.SetupSummary)                          {}
func (reporter *JUnitReporter) SuiteDidEnd(_ *types.SuiteSummary)                               {}
