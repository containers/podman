package reports

type RmReport struct {
	Id  string `json:"Id"` //nolint
	Err error  `json:"Err,omitempty"`
}

func RmReportsIds(r []*RmReport) []string {
	ids := make([]string, 0, len(r))
	for _, v := range r {
		if v == nil || v.Id == "" {
			continue
		}
		ids = append(ids, v.Id)
	}
	return ids
}

func RmReportsErrs(r []*RmReport) []error {
	errs := make([]error, 0, len(r))
	for _, v := range r {
		if v == nil || v.Err == nil {
			continue
		}
		errs = append(errs, v.Err)
	}
	return errs
}
