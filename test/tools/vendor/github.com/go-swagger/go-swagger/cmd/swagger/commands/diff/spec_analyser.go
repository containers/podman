package diff

import (
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
)

// StringType For identifying string types
const StringType = "string"

// URLMethodResponse encapsulates these three elements to act as a map key
type URLMethodResponse struct {
	Path     string `json:"path"`
	Method   string `json:"method"`
	Response string `json:"response"`
}

// MarshalText - for serializing as a map key
func (p URLMethod) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%s %s", p.Path, p.Method)), nil
}

// URLMethods allows iteration of endpoints based on url and method
type URLMethods map[URLMethod]*PathItemOp

// SpecAnalyser contains all the differences for a Spec
type SpecAnalyser struct {
	Diffs                 SpecDifferences
	urlMethods1           URLMethods
	urlMethods2           URLMethods
	Definitions1          spec.Definitions
	Definitions2          spec.Definitions
	Info1                 *spec.Info
	Info2                 *spec.Info
	ReferencedDefinitions map[string]bool

	schemasCompared map[string]struct{}
}

// NewSpecAnalyser returns an empty SpecDiffs
func NewSpecAnalyser() *SpecAnalyser {
	return &SpecAnalyser{
		Diffs:                 SpecDifferences{},
		ReferencedDefinitions: map[string]bool{},
	}
}

// Analyse the differences in two specs
func (sd *SpecAnalyser) Analyse(spec1, spec2 *spec.Swagger) error {
	sd.schemasCompared = make(map[string]struct{})
	sd.Definitions1 = spec1.Definitions
	sd.Definitions2 = spec2.Definitions
	sd.Info1 = spec1.Info
	sd.Info2 = spec2.Info
	sd.urlMethods1 = getURLMethodsFor(spec1)
	sd.urlMethods2 = getURLMethodsFor(spec2)

	sd.analyseSpecMetadata(spec1, spec2)
	sd.analyseEndpoints()
	sd.analyseRequestParams()
	sd.analyseEndpointData()
	sd.analyseResponseParams()
	sd.analyseExtensions(spec1, spec2)
	sd.AnalyseDefinitions()

	return nil
}

func (sd *SpecAnalyser) analyseSpecMetadata(spec1, spec2 *spec.Swagger) {
	// breaking if it no longer consumes any formats
	added, deleted, _ := fromStringArray(spec1.Consumes).DiffsTo(spec2.Consumes)

	node := getNameOnlyDiffNode("Spec")
	location := DifferenceLocation{Node: node}
	consumesLoation := location.AddNode(getNameOnlyDiffNode("consumes"))

	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(
			SpecDifference{DifferenceLocation: consumesLoation, Code: AddedConsumesFormat, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: consumesLoation, Code: DeletedConsumesFormat, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// // breaking if it no longer produces any formats
	added, deleted, _ = fromStringArray(spec1.Produces).DiffsTo(spec2.Produces)
	producesLocation := location.AddNode(getNameOnlyDiffNode("produces"))
	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: producesLocation, Code: AddedProducesFormat, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: producesLocation, Code: DeletedProducesFormat, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// // breaking if it no longer supports a scheme
	added, deleted, _ = fromStringArray(spec1.Schemes).DiffsTo(spec2.Schemes)
	schemesLocation := location.AddNode(getNameOnlyDiffNode("schemes"))

	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: schemesLocation, Code: AddedSchemes, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: schemesLocation, Code: DeletedSchemes, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// host should be able to change without any issues?
	sd.analyseMetaDataProperty(spec1.Info.Description, spec2.Info.Description, ChangedDescripton, NonBreaking)

	// // host should be able to change without any issues?
	sd.analyseMetaDataProperty(spec1.Host, spec2.Host, ChangedHostURL, Breaking)
	// sd.Host = compareStrings(spec1.Host, spec2.Host)

	// // Base Path change will break non generated clients
	sd.analyseMetaDataProperty(spec1.BasePath, spec2.BasePath, ChangedBasePath, Breaking)

	// TODO: what to do about security?
	// Missing security scheme will break a client
	// Security            []map[string][]string  `json:"security,omitempty"`
	// Tags                []Tag                  `json:"tags,omitempty"`
	// ExternalDocs        *ExternalDocumentation `json:"externalDocs,omitempty"`
}

func (sd *SpecAnalyser) analyseEndpoints() {
	sd.findDeletedEndpoints()
	sd.findAddedEndpoints()
}

// AnalyseDefinitions check for changes to definition objects not referenced in any endpoint
func (sd *SpecAnalyser) AnalyseDefinitions() {
	alreadyReferenced := map[string]bool{}
	for k := range sd.ReferencedDefinitions {
		alreadyReferenced[k] = true
	}
	location := DifferenceLocation{Node: &Node{Field: "Spec Definitions"}}
	for name1, sch := range sd.Definitions1 {
		schema1 := sch
		if _, ok := alreadyReferenced[name1]; !ok {
			childLocation := location.AddNode(&Node{Field: name1})
			if schema2, ok := sd.Definitions2[name1]; ok {
				sd.compareSchema(childLocation, &schema1, &schema2)
			} else {
				sd.addDiffs(childLocation, []TypeDiff{{Change: DeletedDefinition}})
			}
		}
	}
	for name2 := range sd.Definitions2 {
		if _, ok := sd.Definitions1[name2]; !ok {
			childLocation := location.AddNode(&Node{Field: name2})
			sd.addDiffs(childLocation, []TypeDiff{{Change: AddedDefinition}})
		}
	}
}

func (sd *SpecAnalyser) analyseEndpointData() {

	for URLMethod, op2 := range sd.urlMethods2 {
		if op1, ok := sd.urlMethods1[URLMethod]; ok {
			addedTags, deletedTags, _ := fromStringArray(op1.Operation.Tags).DiffsTo(op2.Operation.Tags)
			location := DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method}

			for _, eachAddedTag := range addedTags {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: AddedTag, DiffInfo: fmt.Sprintf(`"%s"`, eachAddedTag)})
			}
			for _, eachDeletedTag := range deletedTags {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: DeletedTag, DiffInfo: fmt.Sprintf(`"%s"`, eachDeletedTag)})
			}

			sd.compareDescripton(location, op1.Operation.Description, op2.Operation.Description)

		}
	}
}

func (sd *SpecAnalyser) analyseRequestParams() {
	locations := []string{"query", "path", "body", "header", "formData"}

	for _, paramLocation := range locations {
		rootNode := getNameOnlyDiffNode(strings.Title(paramLocation))
		for URLMethod, op2 := range sd.urlMethods2 {
			if op1, ok := sd.urlMethods1[URLMethod]; ok {

				params1 := getParams(op1.ParentPathItem.Parameters, op1.Operation.Parameters, paramLocation)
				params2 := getParams(op2.ParentPathItem.Parameters, op2.Operation.Parameters, paramLocation)

				location := DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method, Node: rootNode}

				// detect deleted params
				for paramName1, param1 := range params1 {
					if _, ok := params2[paramName1]; !ok {
						childLocation := location.AddNode(getSchemaDiffNode(paramName1, &param1.SimpleSchema))
						code := DeletedOptionalParam
						if param1.Required {
							code = DeletedRequiredParam
						}
						sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLocation, Code: code})
					}
				}
				// detect added changed params
				for paramName2, param2 := range params2 {
					// changed?
					if param1, ok := params1[paramName2]; ok {
						sd.compareParams(URLMethod, paramLocation, paramName2, param1, param2)
					} else {
						// Added
						childLocation := location.AddNode(getSchemaDiffNode(paramName2, &param2.SimpleSchema))
						code := AddedOptionalParam
						if param2.Required {
							code = AddedRequiredParam
						}
						sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLocation, Code: code})
					}
				}
			}
		}
	}
}

func (sd *SpecAnalyser) analyseResponseParams() {
	// Loop through url+methods in spec 2 - check deleted and changed
	for eachURLMethodFrom2, op2 := range sd.urlMethods2 {

		// present in both specs? Use key from spec 2 to lookup in spec 1
		if op1, ok := sd.urlMethods1[eachURLMethodFrom2]; ok {
			// compare responses for url and method
			op1Responses := op1.Operation.Responses.StatusCodeResponses
			op2Responses := op2.Operation.Responses.StatusCodeResponses

			// deleted responses
			for code1 := range op1Responses {
				if _, ok := op2Responses[code1]; !ok {
					location := DifferenceLocation{URL: eachURLMethodFrom2.Path, Method: eachURLMethodFrom2.Method, Response: code1, Node: getSchemaDiffNode("Body", op1Responses[code1].Schema)}
					sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: DeletedResponse})
				}
			}
			// Added updated Response Codes
			for code2, op2Response := range op2Responses {
				if op1Response, ok := op1Responses[code2]; ok {
					op1Headers := op1Response.ResponseProps.Headers
					headerRootNode := getNameOnlyDiffNode("Headers")

					// Iterate Spec2 Headers looking for added and updated
					location := DifferenceLocation{URL: eachURLMethodFrom2.Path, Method: eachURLMethodFrom2.Method, Response: code2, Node: headerRootNode}
					for op2HeaderName, op2Header := range op2Response.ResponseProps.Headers {
						if op1Header, ok := op1Headers[op2HeaderName]; ok {
							diffs := sd.CompareProps(forHeader(op1Header), forHeader(op2Header))
							sd.addDiffs(location, diffs)
						} else {
							sd.Diffs = sd.Diffs.addDiff(SpecDifference{
								DifferenceLocation: location.AddNode(getSchemaDiffNode(op2HeaderName, &op2Header.SimpleSchema)),
								Code:               AddedResponseHeader})
						}
					}
					for op1HeaderName := range op1Response.ResponseProps.Headers {
						if _, ok := op2Response.ResponseProps.Headers[op1HeaderName]; !ok {
							op1Header := op1Response.ResponseProps.Headers[op1HeaderName]
							sd.Diffs = sd.Diffs.addDiff(SpecDifference{
								DifferenceLocation: location.AddNode(getSchemaDiffNode(op1HeaderName, &op1Header.SimpleSchema)),
								Code:               DeletedResponseHeader})
						}
					}
					schem := op1Response.Schema
					node := getNameOnlyDiffNode("NoContent")
					if schem != nil {
						node = getSchemaDiffNode("Body", &schem.SchemaProps)
					}
					responseLocation := DifferenceLocation{URL: eachURLMethodFrom2.Path,
						Method:   eachURLMethodFrom2.Method,
						Response: code2,
						Node:     node}
					sd.compareDescripton(responseLocation, op1Response.Description, op2Response.Description)

					if op1Response.Schema != nil {
						sd.compareSchema(
							DifferenceLocation{URL: eachURLMethodFrom2.Path, Method: eachURLMethodFrom2.Method, Response: code2, Node: getSchemaDiffNode("Body", op1Response.Schema)},
							op1Response.Schema,
							op2Response.Schema)
					}
				} else {
					// op2Response
					sd.Diffs = sd.Diffs.addDiff(SpecDifference{
						DifferenceLocation: DifferenceLocation{URL: eachURLMethodFrom2.Path, Method: eachURLMethodFrom2.Method, Response: code2, Node: getSchemaDiffNode("Body", op2Response.Schema)},
						Code:               AddedResponse})
				}
			}
		}
	}
}

func (sd *SpecAnalyser) analyseExtensions(spec1, spec2 *spec.Swagger) {
	// root
	specLoc := DifferenceLocation{Node: &Node{Field: "Spec"}}
	sd.checkAddedExtensions(spec1.Extensions, spec2.Extensions, specLoc, "")
	sd.checkDeletedExtensions(spec1.Extensions, spec2.Extensions, specLoc, "")

	sd.analyzeInfoExtensions()
	sd.analyzeTagExtensions(spec1, spec2)
	sd.analyzeSecurityDefinitionExtensions(spec1, spec2)

	sd.analyzeOperationExtensions()
}

func (sd *SpecAnalyser) analyzeOperationExtensions() {
	for urlMethod, op2 := range sd.urlMethods2 {
		pathAndMethodLoc := DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method}
		if op1, ok := sd.urlMethods1[urlMethod]; ok {
			sd.checkAddedExtensions(op1.Extensions, op2.Extensions, DifferenceLocation{URL: urlMethod.Path}, "")
			sd.checkAddedExtensions(op1.Operation.Responses.Extensions, op2.Operation.Responses.Extensions, pathAndMethodLoc, "Responses")
			sd.checkAddedExtensions(op1.Operation.Extensions, op2.Operation.Extensions, pathAndMethodLoc, "")

			for code, resp := range op1.Operation.Responses.StatusCodeResponses {
				for hdr, h := range resp.Headers {
					op2StatusCode, ok := op2.Operation.Responses.StatusCodeResponses[code]
					if ok {
						if _, ok = op2StatusCode.Headers[hdr]; ok {
							sd.checkAddedExtensions(h.Extensions, op2StatusCode.Headers[hdr].Extensions, DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method, Node: getNameOnlyDiffNode("Headers")}, hdr)
						}
					}
				}

				resp2 := op2.Operation.Responses.StatusCodeResponses[code]
				sd.analyzeSchemaExtensions(resp.Schema, resp2.Schema, code, urlMethod)
			}

		}
	}

	for urlMethod, op1 := range sd.urlMethods1 {
		pathAndMethodLoc := DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method}
		if op2, ok := sd.urlMethods2[urlMethod]; ok {
			sd.checkDeletedExtensions(op1.Extensions, op2.Extensions, DifferenceLocation{URL: urlMethod.Path}, "")
			sd.checkDeletedExtensions(op1.Operation.Responses.Extensions, op2.Operation.Responses.Extensions, pathAndMethodLoc, "Responses")
			sd.checkDeletedExtensions(op1.Operation.Extensions, op2.Operation.Extensions, pathAndMethodLoc, "")
			for code, resp := range op1.Operation.Responses.StatusCodeResponses {
				for hdr, h := range resp.Headers {
					op2StatusCode, ok := op2.Operation.Responses.StatusCodeResponses[code]
					if ok {
						if _, ok = op2StatusCode.Headers[hdr]; ok {
							sd.checkDeletedExtensions(h.Extensions, op2StatusCode.Headers[hdr].Extensions, DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method, Node: getNameOnlyDiffNode("Headers")}, hdr)
						}
					}
				}
			}
		}
	}
}

func (sd *SpecAnalyser) analyzeSecurityDefinitionExtensions(spec1 *spec.Swagger, spec2 *spec.Swagger) {
	securityDefLoc := DifferenceLocation{Node: &Node{Field: "Security Definitions"}}
	for key, securityDef := range spec1.SecurityDefinitions {
		if securityDef2, ok := spec2.SecurityDefinitions[key]; ok {
			sd.checkAddedExtensions(securityDef.Extensions, securityDef2.Extensions, securityDefLoc, "")
		}
	}

	for key, securityDef := range spec2.SecurityDefinitions {
		if securityDef1, ok := spec1.SecurityDefinitions[key]; ok {
			sd.checkDeletedExtensions(securityDef1.Extensions, securityDef.Extensions, securityDefLoc, "")
		}
	}
}

func (sd *SpecAnalyser) analyzeSchemaExtensions(schema1, schema2 *spec.Schema, code int, urlMethod URLMethod) {
	if schema1 != nil && schema2 != nil {
		diffLoc := DifferenceLocation{Response: code, URL: urlMethod.Path, Method: urlMethod.Method, Node: getSchemaDiffNode("Body", schema2)}
		sd.checkAddedExtensions(schema1.Extensions, schema2.Extensions, diffLoc, "")
		sd.checkDeletedExtensions(schema1.Extensions, schema2.Extensions, diffLoc, "")
		if schema1.Items != nil && schema2.Items != nil {
			sd.analyzeSchemaExtensions(schema1.Items.Schema, schema2.Items.Schema, code, urlMethod)
			for i := range schema1.Items.Schemas {
				s1 := schema1.Items.Schemas[i]
				for j := range schema2.Items.Schemas {
					s2 := schema2.Items.Schemas[j]
					sd.analyzeSchemaExtensions(&s1, &s2, code, urlMethod)
				}
			}
		}
	}
}

func (sd *SpecAnalyser) analyzeInfoExtensions() {
	if sd.Info1 != nil && sd.Info2 != nil {
		diffLocation := DifferenceLocation{Node: &Node{Field: "Spec Info"}}
		sd.checkAddedExtensions(sd.Info1.Extensions, sd.Info2.Extensions, diffLocation, "")
		sd.checkDeletedExtensions(sd.Info1.Extensions, sd.Info2.Extensions, diffLocation, "")
		if sd.Info1.Contact != nil && sd.Info2.Contact != nil {
			diffLocation = DifferenceLocation{Node: &Node{Field: "Spec Info.Contact"}}
			sd.checkAddedExtensions(sd.Info1.Contact.Extensions, sd.Info2.Contact.Extensions, diffLocation, "")
			sd.checkDeletedExtensions(sd.Info1.Contact.Extensions, sd.Info2.Contact.Extensions, diffLocation, "")
		}
		if sd.Info1.License != nil && sd.Info2.License != nil {
			diffLocation = DifferenceLocation{Node: &Node{Field: "Spec Info.License"}}
			sd.checkAddedExtensions(sd.Info1.License.Extensions, sd.Info2.License.Extensions, diffLocation, "")
			sd.checkDeletedExtensions(sd.Info1.License.Extensions, sd.Info2.License.Extensions, diffLocation, "")
		}
	}
}

func (sd *SpecAnalyser) analyzeTagExtensions(spec1 *spec.Swagger, spec2 *spec.Swagger) {
	diffLocation := DifferenceLocation{Node: &Node{Field: "Spec Tags"}}
	for _, spec2Tag := range spec2.Tags {
		for _, spec1Tag := range spec1.Tags {
			if spec2Tag.Name == spec1Tag.Name {
				sd.checkAddedExtensions(spec1Tag.Extensions, spec2Tag.Extensions, diffLocation, "")
			}
		}
	}
	for _, spec1Tag := range spec1.Tags {
		for _, spec2Tag := range spec2.Tags {
			if spec1Tag.Name == spec2Tag.Name {
				sd.checkDeletedExtensions(spec1Tag.Extensions, spec2Tag.Extensions, diffLocation, "")
			}
		}
	}
}

func (sd *SpecAnalyser) checkAddedExtensions(extensions1 spec.Extensions, extensions2 spec.Extensions, diffLocation DifferenceLocation, fieldPrefix string) {
	for extKey := range extensions2 {
		if _, ok := extensions1[extKey]; !ok {
			if fieldPrefix != "" {
				extKey = fmt.Sprintf("%s.%s", fieldPrefix, extKey)
			}
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{
				DifferenceLocation: diffLocation.AddNode(&Node{Field: extKey}),
				Code:               AddedExtension,
				Compatibility:      Warning, // this could potentially be a breaking change
			})
		}
	}
}

func (sd *SpecAnalyser) checkDeletedExtensions(extensions1 spec.Extensions, extensions2 spec.Extensions, diffLocation DifferenceLocation, fieldPrefix string) {
	for extKey := range extensions1 {
		if _, ok := extensions2[extKey]; !ok {
			if fieldPrefix != "" {
				extKey = fmt.Sprintf("%s.%s", fieldPrefix, extKey)
			}
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{
				DifferenceLocation: diffLocation.AddNode(&Node{Field: extKey}),
				Code:               DeletedExtension,
				Compatibility:      Warning, // this could potentially be a breaking change
			})
		}
	}
}

func addTypeDiff(diffs []TypeDiff, diff TypeDiff) []TypeDiff {
	if diff.Change != NoChangeDetected {
		diffs = append(diffs, diff)
	}
	return diffs
}

// CompareProps computes type specific property diffs
func (sd *SpecAnalyser) CompareProps(type1, type2 *spec.SchemaProps) []TypeDiff {

	diffs := []TypeDiff{}

	diffs = CheckToFromPrimitiveType(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	if isArray(type1) {
		maxItemDiffs := CompareIntValues("MaxItems", type1.MaxItems, type2.MaxItems, WidenedType, NarrowedType)
		diffs = append(diffs, maxItemDiffs...)
		minItemsDiff := CompareIntValues("MinItems", type1.MinItems, type2.MinItems, NarrowedType, WidenedType)
		diffs = append(diffs, minItemsDiff...)
	}

	if len(diffs) > 0 {
		return diffs
	}

	diffs = CheckRefChange(diffs, type1, type2)
	if len(diffs) > 0 {
		return diffs
	}

	if !(isPrimitiveType(type1.Type) && isPrimitiveType(type2.Type)) {
		return diffs
	}

	// check primitive type hierarchy change eg string -> integer = NarrowedChange
	if type1.Type[0] != type2.Type[0] ||
		type1.Format != type2.Format {
		diff := getTypeHierarchyChange(primitiveTypeString(type1.Type[0], type1.Format), primitiveTypeString(type2.Type[0], type2.Format))
		diffs = addTypeDiff(diffs, diff)
	}

	diffs = CheckStringTypeChanges(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	diffs = checkNumericTypeChanges(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	return diffs
}

func (sd *SpecAnalyser) compareParams(urlMethod URLMethod, location string, name string, param1, param2 spec.Parameter) {
	diffLocation := DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method}

	childLocation := diffLocation.AddNode(getNameOnlyDiffNode(strings.Title(location)))
	paramLocation := diffLocation.AddNode(getNameOnlyDiffNode(name))
	sd.compareDescripton(paramLocation, param1.Description, param2.Description)

	if param1.Schema != nil && param2.Schema != nil {
		if len(name) > 0 {
			childLocation = childLocation.AddNode(getSchemaDiffNode(name, param2.Schema))
		}
		sd.compareSchema(childLocation, param1.Schema, param2.Schema)
	}

	diffs := sd.CompareProps(forParam(param1), forParam(param2))

	childLocation = childLocation.AddNode(getSchemaDiffNode(name, &param2.SimpleSchema))
	if len(diffs) > 0 {
		sd.addDiffs(childLocation, diffs)
	}

	diffs = CheckToFromRequired(param1.Required, param2.Required)
	if len(diffs) > 0 {
		sd.addDiffs(childLocation, diffs)
	}

	sd.compareSimpleSchema(childLocation, &param1.SimpleSchema, &param2.SimpleSchema)
}

func (sd *SpecAnalyser) addTypeDiff(location DifferenceLocation, diff *TypeDiff) {
	diffCopy := diff
	desc := diffCopy.Description
	if len(desc) == 0 {
		if diffCopy.FromType != diffCopy.ToType {
			desc = fmt.Sprintf("%s -> %s", diffCopy.FromType, diffCopy.ToType)
		}
	}
	sd.Diffs = sd.Diffs.addDiff(SpecDifference{
		DifferenceLocation: location,
		Code:               diffCopy.Change,
		DiffInfo:           desc})
}

func (sd *SpecAnalyser) compareDescripton(location DifferenceLocation, desc1, desc2 string) {
	if desc1 != desc2 {
		code := ChangedDescripton
		if len(desc1) > 0 {
			code = DeletedDescripton
		} else if len(desc2) > 0 {
			code = AddedDescripton
		}
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: code})
	}
}

func isPrimitiveType(item spec.StringOrArray) bool {
	return len(item) > 0 && item[0] != ArrayType && item[0] != ObjectType
}

func isArrayType(item spec.StringOrArray) bool {
	return len(item) > 0 && item[0] == ArrayType
}
func (sd *SpecAnalyser) getRefSchemaFromSpec1(ref spec.Ref) (*spec.Schema, string) {
	return sd.schemaFromRef(ref, &sd.Definitions1)
}

func (sd *SpecAnalyser) getRefSchemaFromSpec2(ref spec.Ref) (*spec.Schema, string) {
	return sd.schemaFromRef(ref, &sd.Definitions2)
}

// CompareSchemaFn Fn spec for comparing schemas
type CompareSchemaFn func(location DifferenceLocation, schema1, schema2 *spec.Schema)

func (sd *SpecAnalyser) compareSchema(location DifferenceLocation, schema1, schema2 *spec.Schema) {

	refDiffs := []TypeDiff{}
	refDiffs = CheckRefChange(refDiffs, schema1, schema2)
	if len(refDiffs) > 0 {
		for _, d := range refDiffs {
			diff := d
			sd.addTypeDiff(location, &diff)
		}
		return
	}

	if isRefType(schema1) {
		key := schemaLocationKey(location)
		if _, ok := sd.schemasCompared[key]; ok {
			return
		}
		sd.schemasCompared[key] = struct{}{}
		schema1, _ = sd.schemaFromRef(getRef(schema1), &sd.Definitions1)
	}

	if isRefType(schema2) {
		schema2, _ = sd.schemaFromRef(getRef(schema2), &sd.Definitions2)
	}

	sd.compareDescripton(location, schema1.Description, schema2.Description)

	typeDiffs := sd.CompareProps(&schema1.SchemaProps, &schema2.SchemaProps)
	if len(typeDiffs) > 0 {
		sd.addDiffs(location, typeDiffs)
		return
	}

	if isArray(schema1) {
		if isArray(schema2) {
			sd.compareSchema(location, schema1.Items.Schema, schema2.Items.Schema)
		} else {
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedType, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		}
	}

	diffs := CompareProperties(location, schema1, schema2, sd.getRefSchemaFromSpec1, sd.getRefSchemaFromSpec2, sd.compareSchema)
	for _, diff := range diffs {
		sd.Diffs = sd.Diffs.addDiff(diff)
	}
}

func (sd *SpecAnalyser) compareSimpleSchema(location DifferenceLocation, schema1, schema2 *spec.SimpleSchema) {
	// check optional/required
	if schema1.Nullable != schema2.Nullable {
		// If optional is made required
		if schema1.Nullable && !schema2.Nullable {
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedOptionalToRequired, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		} else if !schema1.Nullable && schema2.Nullable {
			// If required is made optional
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedRequiredToOptional, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		}
	}

	if schema1.CollectionFormat != schema2.CollectionFormat {
		sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedCollectionFormat, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
	}

	if schema1.Default != schema2.Default {
		switch {
		case schema1.Default == nil && schema2.Default != nil:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: AddedDefault, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		case schema1.Default != nil && schema2.Default == nil:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: DeletedDefault, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		default:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedDefault, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		}
	}

	if schema1.Example != schema2.Example {
		switch {
		case schema1.Example == nil && schema2.Example != nil:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: AddedExample, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		case schema1.Example != nil && schema2.Example == nil:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: DeletedExample, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		default:
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedExample, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		}
	}

	if isArray(schema1) {
		if isArray(schema2) {
			sd.compareSimpleSchema(location, &schema1.Items.SimpleSchema, &schema2.Items.SimpleSchema)
		} else {
			sd.addDiffs(location, addTypeDiff([]TypeDiff{}, TypeDiff{Change: ChangedType, FromType: getSchemaTypeStr(schema1), ToType: getSchemaTypeStr(schema2)}))
		}
	}
}

func (sd *SpecAnalyser) addDiffs(location DifferenceLocation, diffs []TypeDiff) {
	for _, e := range diffs {
		eachTypeDiff := e
		if eachTypeDiff.Change != NoChangeDetected {
			sd.addTypeDiff(location, &eachTypeDiff)
		}
	}
}

func addChildDiffNode(location DifferenceLocation, propName string, propSchema *spec.Schema) DifferenceLocation {
	newNode := location.Node
	childNode := fromSchemaProps(propName, &propSchema.SchemaProps)
	if newNode != nil {
		newNode = newNode.Copy()
		newNode.AddLeafNode(&childNode)
	} else {
		newNode = &childNode
	}
	return DifferenceLocation{
		URL:      location.URL,
		Method:   location.Method,
		Response: location.Response,
		Node:     newNode,
	}
}

func fromSchemaProps(fieldName string, props *spec.SchemaProps) Node {
	node := Node{}
	node.TypeName, node.IsArray = getSchemaType(props)
	node.Field = fieldName
	return node
}

func (sd *SpecAnalyser) findAddedEndpoints() {
	for URLMethod := range sd.urlMethods2 {
		if _, ok := sd.urlMethods1[URLMethod]; !ok {
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method}, Code: AddedEndpoint})
		}
	}
}

func (sd *SpecAnalyser) findDeletedEndpoints() {
	for eachURLMethod, operation1 := range sd.urlMethods1 {
		code := DeletedEndpoint
		if (operation1.ParentPathItem.Options != nil && operation1.ParentPathItem.Options.Deprecated) ||
			(operation1.Operation.Deprecated) {
			code = DeletedDeprecatedEndpoint
		}
		if _, ok := sd.urlMethods2[eachURLMethod]; !ok {
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{URL: eachURLMethod.Path, Method: eachURLMethod.Method}, Code: code})
		}
	}
}

func (sd *SpecAnalyser) analyseMetaDataProperty(item1, item2 string, codeIfDiff SpecChangeCode, compatIfDiff Compatibility) {
	if item1 != item2 {
		diffSpec := fmt.Sprintf("%s -> %s", item1, item2)
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{Node: &Node{Field: "Spec Metadata"}}, Code: codeIfDiff, Compatibility: compatIfDiff, DiffInfo: diffSpec})
	}
}

func (sd *SpecAnalyser) schemaFromRef(ref spec.Ref, defns *spec.Definitions) (actualSchema *spec.Schema, definitionName string) {
	definitionName = definitionFromRef(ref)
	foundSchema, ok := (*defns)[definitionName]
	if !ok {
		return nil, definitionName
	}
	sd.ReferencedDefinitions[definitionName] = true
	actualSchema = &foundSchema
	return

}

func schemaLocationKey(location DifferenceLocation) string {
	return location.Method + location.URL + location.Node.Field + location.Node.TypeName
}

// PropertyDefn combines a property with its required-ness
type PropertyDefn struct {
	Schema   *spec.Schema
	Required bool
}

// PropertyMap a unified map including all AllOf fields
type PropertyMap map[string]PropertyDefn
