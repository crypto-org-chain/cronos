package events

type (
	ValueDecoder  func(attributeValue string, indexed bool) (ethPrimitives []any, err error)
	ValueDecoders map[string]ValueDecoder
)
