//
//Copyright 2021 The Dapr Authors
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//http://www.apache.org/licenses/LICENSE-2.0
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: dapr/proto/sentry/v1/sentry.proto

package sentry

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SignCertificateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id          string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Token       string `protobuf:"bytes,2,opt,name=token,proto3" json:"token,omitempty"`
	TrustDomain string `protobuf:"bytes,3,opt,name=trust_domain,json=trustDomain,proto3" json:"trust_domain,omitempty"`
	Namespace   string `protobuf:"bytes,4,opt,name=namespace,proto3" json:"namespace,omitempty"`
	// A PEM-encoded x509 CSR.
	CertificateSigningRequest []byte `protobuf:"bytes,5,opt,name=certificate_signing_request,json=certificateSigningRequest,proto3" json:"certificate_signing_request,omitempty"`
	// Name of the validator to use, if not the default for the environemtn.
	TokenValidator string `protobuf:"bytes,6,opt,name=token_validator,json=tokenValidator,proto3" json:"token_validator,omitempty"`
}

func (x *SignCertificateRequest) Reset() {
	*x = SignCertificateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dapr_proto_sentry_v1_sentry_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignCertificateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignCertificateRequest) ProtoMessage() {}

func (x *SignCertificateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_dapr_proto_sentry_v1_sentry_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignCertificateRequest.ProtoReflect.Descriptor instead.
func (*SignCertificateRequest) Descriptor() ([]byte, []int) {
	return file_dapr_proto_sentry_v1_sentry_proto_rawDescGZIP(), []int{0}
}

func (x *SignCertificateRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *SignCertificateRequest) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

func (x *SignCertificateRequest) GetTrustDomain() string {
	if x != nil {
		return x.TrustDomain
	}
	return ""
}

func (x *SignCertificateRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *SignCertificateRequest) GetCertificateSigningRequest() []byte {
	if x != nil {
		return x.CertificateSigningRequest
	}
	return nil
}

func (x *SignCertificateRequest) GetTokenValidator() string {
	if x != nil {
		return x.TokenValidator
	}
	return ""
}

type SignCertificateResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// A PEM-encoded x509 Certificate.
	WorkloadCertificate []byte `protobuf:"bytes,1,opt,name=workload_certificate,json=workloadCertificate,proto3" json:"workload_certificate,omitempty"`
	// A list of PEM-encoded x509 Certificates that establish the trust chain
	// between the workload certificate and the well-known trust root cert.
	TrustChainCertificates [][]byte               `protobuf:"bytes,2,rep,name=trust_chain_certificates,json=trustChainCertificates,proto3" json:"trust_chain_certificates,omitempty"`
	ValidUntil             *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=valid_until,json=validUntil,proto3" json:"valid_until,omitempty"`
}

func (x *SignCertificateResponse) Reset() {
	*x = SignCertificateResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dapr_proto_sentry_v1_sentry_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignCertificateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignCertificateResponse) ProtoMessage() {}

func (x *SignCertificateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dapr_proto_sentry_v1_sentry_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignCertificateResponse.ProtoReflect.Descriptor instead.
func (*SignCertificateResponse) Descriptor() ([]byte, []int) {
	return file_dapr_proto_sentry_v1_sentry_proto_rawDescGZIP(), []int{1}
}

func (x *SignCertificateResponse) GetWorkloadCertificate() []byte {
	if x != nil {
		return x.WorkloadCertificate
	}
	return nil
}

func (x *SignCertificateResponse) GetTrustChainCertificates() [][]byte {
	if x != nil {
		return x.TrustChainCertificates
	}
	return nil
}

func (x *SignCertificateResponse) GetValidUntil() *timestamppb.Timestamp {
	if x != nil {
		return x.ValidUntil
	}
	return nil
}

var File_dapr_proto_sentry_v1_sentry_proto protoreflect.FileDescriptor

var file_dapr_proto_sentry_v1_sentry_proto_rawDesc = []byte{
	0x0a, 0x21, 0x64, 0x61, 0x70, 0x72, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x65, 0x6e,
	0x74, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x2f, 0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x14, 0x64, 0x61, 0x70, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e,
	0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xe8, 0x01, 0x0a, 0x16, 0x53,
	0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x21, 0x0a, 0x0c, 0x74,
	0x72, 0x75, 0x73, 0x74, 0x5f, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x74, 0x72, 0x75, 0x73, 0x74, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x1c,
	0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x3e, 0x0a, 0x1b,
	0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x5f, 0x73, 0x69, 0x67, 0x6e,
	0x69, 0x6e, 0x67, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x19, 0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x53, 0x69,
	0x67, 0x6e, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x0f,
	0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x5f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x56, 0x61, 0x6c, 0x69,
	0x64, 0x61, 0x74, 0x6f, 0x72, 0x22, 0xc3, 0x01, 0x0a, 0x17, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65,
	0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x31, 0x0a, 0x14, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x63, 0x65,
	0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x13, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69,
	0x63, 0x61, 0x74, 0x65, 0x12, 0x38, 0x0a, 0x18, 0x74, 0x72, 0x75, 0x73, 0x74, 0x5f, 0x63, 0x68,
	0x61, 0x69, 0x6e, 0x5f, 0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x16, 0x74, 0x72, 0x75, 0x73, 0x74, 0x43, 0x68, 0x61,
	0x69, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x73, 0x12, 0x3b,
	0x0a, 0x0b, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x5f, 0x75, 0x6e, 0x74, 0x69, 0x6c, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x0a, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x55, 0x6e, 0x74, 0x69, 0x6c, 0x32, 0x76, 0x0a, 0x02, 0x43,
	0x41, 0x12, 0x70, 0x0a, 0x0f, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69,
	0x63, 0x61, 0x74, 0x65, 0x12, 0x2c, 0x2e, 0x64, 0x61, 0x70, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2e, 0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x69, 0x67, 0x6e,
	0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x2d, 0x2e, 0x64, 0x61, 0x70, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e,
	0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x43, 0x65,
	0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x42, 0x31, 0x5a, 0x2f, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x64, 0x61, 0x70, 0x72, 0x2f, 0x64, 0x61, 0x70, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x3b,
	0x73, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_dapr_proto_sentry_v1_sentry_proto_rawDescOnce sync.Once
	file_dapr_proto_sentry_v1_sentry_proto_rawDescData = file_dapr_proto_sentry_v1_sentry_proto_rawDesc
)

func file_dapr_proto_sentry_v1_sentry_proto_rawDescGZIP() []byte {
	file_dapr_proto_sentry_v1_sentry_proto_rawDescOnce.Do(func() {
		file_dapr_proto_sentry_v1_sentry_proto_rawDescData = protoimpl.X.CompressGZIP(file_dapr_proto_sentry_v1_sentry_proto_rawDescData)
	})
	return file_dapr_proto_sentry_v1_sentry_proto_rawDescData
}

var file_dapr_proto_sentry_v1_sentry_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_dapr_proto_sentry_v1_sentry_proto_goTypes = []interface{}{
	(*SignCertificateRequest)(nil),  // 0: dapr.proto.sentry.v1.SignCertificateRequest
	(*SignCertificateResponse)(nil), // 1: dapr.proto.sentry.v1.SignCertificateResponse
	(*timestamppb.Timestamp)(nil),   // 2: google.protobuf.Timestamp
}
var file_dapr_proto_sentry_v1_sentry_proto_depIdxs = []int32{
	2, // 0: dapr.proto.sentry.v1.SignCertificateResponse.valid_until:type_name -> google.protobuf.Timestamp
	0, // 1: dapr.proto.sentry.v1.CA.SignCertificate:input_type -> dapr.proto.sentry.v1.SignCertificateRequest
	1, // 2: dapr.proto.sentry.v1.CA.SignCertificate:output_type -> dapr.proto.sentry.v1.SignCertificateResponse
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_dapr_proto_sentry_v1_sentry_proto_init() }
func file_dapr_proto_sentry_v1_sentry_proto_init() {
	if File_dapr_proto_sentry_v1_sentry_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_dapr_proto_sentry_v1_sentry_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignCertificateRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_dapr_proto_sentry_v1_sentry_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignCertificateResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_dapr_proto_sentry_v1_sentry_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_dapr_proto_sentry_v1_sentry_proto_goTypes,
		DependencyIndexes: file_dapr_proto_sentry_v1_sentry_proto_depIdxs,
		MessageInfos:      file_dapr_proto_sentry_v1_sentry_proto_msgTypes,
	}.Build()
	File_dapr_proto_sentry_v1_sentry_proto = out.File
	file_dapr_proto_sentry_v1_sentry_proto_rawDesc = nil
	file_dapr_proto_sentry_v1_sentry_proto_goTypes = nil
	file_dapr_proto_sentry_v1_sentry_proto_depIdxs = nil
}
