package ontology

// EqualObservedValues compares exact payloads, not decoder bookkeeping.
func EqualObservedValues(a, b *ObservedValue) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind || a.Encoding != b.Encoding || a.Bytes != b.Bytes ||
		a.ScalarKind != b.ScalarKind || a.Integer != b.Integer || a.FloatBits != b.FloatBits ||
		!equalObservedPointer(a.Text, b.Text) || !equalObservedPointer(a.Bool, b.Bool) {
		return false
	}
	bytesA := a.Bytes != "" || a.hasBytes || (!a.decoded && a.Kind == "bytes")
	bytesB := b.Bytes != "" || b.hasBytes || (!b.decoded && b.Kind == "bytes")
	if bytesA != bytesB || (a.Fields == nil) != (b.Fields == nil) || len(a.Fields) != len(b.Fields) {
		return false
	}
	for key, av := range a.Fields {
		bv, ok := b.Fields[key]
		if !ok || !EqualObservedValues(&av, &bv) {
			return false
		}
	}
	if a.Diagnostic == nil || b.Diagnostic == nil {
		return a.Diagnostic == b.Diagnostic
	}
	ad, bd := a.Diagnostic, b.Diagnostic
	if ad.Code != bd.Code || !equalObservedPointer(ad.Class, bd.Class) ||
		!equalObservedPointer(ad.Reason, bd.Reason) || !equalObservedPointer(ad.Line, bd.Line) {
		return false
	}
	if ad.Span == nil || bd.Span == nil {
		return ad.Span == bd.Span
	}
	return *ad.Span == *bd.Span
}

func equalObservedPointer[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
