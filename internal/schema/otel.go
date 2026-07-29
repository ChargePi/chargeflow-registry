package schema

import "go.opentelemetry.io/otel/attribute"

func ocppVersionAttr(v OCPPVersion) attribute.KeyValue {
	return attribute.String("ocpp.version", string(v))
}

func actionAttr(action string) attribute.KeyValue {
	return attribute.String("ocpp.action", action)
}

func messageTypeAttr(msgType MessageType) attribute.KeyValue {
	return attribute.String("ocpp.message_type", string(msgType))
}

func vendorAttr(vendor string) attribute.KeyValue {
	return attribute.String("ocpp.vendor", vendor)
}
