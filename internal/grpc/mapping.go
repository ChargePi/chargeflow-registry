package grpc

import (
	schemav1 "github.com/ChargePi/chargeflow-registry/gen/proto/schema/v1"
	"github.com/ChargePi/chargeflow-registry/internal/schema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ocppVersionToDomain(v schemav1.OcppVersion) schema.OCPPVersion {
	switch v {
	case schemav1.OcppVersion_OCPP_VERSION_16:
		return schema.OCPPVersion16
	case schemav1.OcppVersion_OCPP_VERSION_201:
		return schema.OCPPVersion201
	case schemav1.OcppVersion_OCPP_VERSION_21:
		return schema.OCPPVersion21
	default:
		return ""
	}
}

func ocppVersionToProto(v schema.OCPPVersion) schemav1.OcppVersion {
	switch v {
	case schema.OCPPVersion16:
		return schemav1.OcppVersion_OCPP_VERSION_16
	case schema.OCPPVersion201:
		return schemav1.OcppVersion_OCPP_VERSION_201
	case schema.OCPPVersion21:
		return schemav1.OcppVersion_OCPP_VERSION_21
	default:
		return schemav1.OcppVersion_OCPP_VERSION_UNSPECIFIED
	}
}

func messageTypeToDomain(t schemav1.MessageType) schema.MessageType {
	switch t {
	case schemav1.MessageType_MESSAGE_TYPE_REQUEST:
		return schema.MessageTypeRequest
	case schemav1.MessageType_MESSAGE_TYPE_RESPONSE:
		return schema.MessageTypeResponse
	default:
		return ""
	}
}

func messageTypeToProto(t schema.MessageType) schemav1.MessageType {
	switch t {
	case schema.MessageTypeRequest:
		return schemav1.MessageType_MESSAGE_TYPE_REQUEST
	case schema.MessageTypeResponse:
		return schemav1.MessageType_MESSAGE_TYPE_RESPONSE
	default:
		return schemav1.MessageType_MESSAGE_TYPE_UNSPECIFIED
	}
}

func schemaStatusToDomain(s schemav1.SchemaStatus) schema.Status {
	switch s {
	case schemav1.SchemaStatus_SCHEMA_STATUS_SUBMITTED:
		return schema.StatusSubmitted
	case schemav1.SchemaStatus_SCHEMA_STATUS_VERIFIED:
		return schema.StatusVerified
	case schemav1.SchemaStatus_SCHEMA_STATUS_REJECTED:
		return schema.StatusRejected
	default:
		return ""
	}
}

func schemaStatusToProto(s schema.Status) schemav1.SchemaStatus {
	switch s {
	case schema.StatusSubmitted:
		return schemav1.SchemaStatus_SCHEMA_STATUS_SUBMITTED
	case schema.StatusVerified:
		return schemav1.SchemaStatus_SCHEMA_STATUS_VERIFIED
	case schema.StatusRejected:
		return schemav1.SchemaStatus_SCHEMA_STATUS_REJECTED
	default:
		return schemav1.SchemaStatus_SCHEMA_STATUS_UNSPECIFIED
	}
}

func schemaToProto(s *schema.Schema) *schemav1.Schema {
	return &schemav1.Schema{
		Id:          s.ID.String(),
		OcppVersion: ocppVersionToProto(s.OCPPVersion),
		Action:      s.Action,
		MessageType: messageTypeToProto(s.MessageType),
		Vendor:      s.Vendor,
		Model:       s.Model,
		Schema:      s.Schema,
		CreatedAt:   timestamppb.New(s.CreatedAt),
		UpdatedAt:   timestamppb.New(s.UpdatedAt),
		Status:      schemaStatusToProto(s.Status),
		Version:     int32(s.Version),
	}
}

func schemasToProto(schemas []*schema.Schema) []*schemav1.Schema {
	out := make([]*schemav1.Schema, len(schemas))
	for i, s := range schemas {
		out[i] = schemaToProto(s)
	}
	return out
}

func schemaVersionToProto(v *schema.SchemaVersion) *schemav1.SchemaVersion {
	return &schemav1.SchemaVersion{
		Version:   int32(v.Version),
		Schema:    v.Schema,
		CreatedAt: timestamppb.New(v.CreatedAt),
	}
}

func schemaVersionsToProto(versions []*schema.SchemaVersion) []*schemav1.SchemaVersion {
	out := make([]*schemav1.SchemaVersion, len(versions))
	for i, v := range versions {
		out[i] = schemaVersionToProto(v)
	}
	return out
}

func vendorModelToProto(vm *schema.VendorModel) *schemav1.VendorModel {
	return &schemav1.VendorModel{
		OcppVersion: ocppVersionToProto(vm.OCPPVersion),
		Vendor:      vm.Vendor,
		Model:       vm.Model,
	}
}

func vendorModelsToProto(vendorModels []*schema.VendorModel) []*schemav1.VendorModel {
	out := make([]*schemav1.VendorModel, len(vendorModels))
	for i, vm := range vendorModels {
		out[i] = vendorModelToProto(vm)
	}
	return out
}
