package validator

// StructValidator is used to validate API structures
type StructValidator interface {
	ValidateStruct(data any, schemaType string) (ok bool, errs []string)
}
