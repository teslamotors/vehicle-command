// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.9
// source: signatures.proto

package signatures

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Tag int32

const (
	Tag_TAG_SIGNATURE_TYPE  Tag = 0
	Tag_TAG_DOMAIN          Tag = 1
	Tag_TAG_PERSONALIZATION Tag = 2
	Tag_TAG_EPOCH           Tag = 3
	Tag_TAG_EXPIRES_AT      Tag = 4
	Tag_TAG_COUNTER         Tag = 5
	Tag_TAG_CHALLENGE       Tag = 6
	Tag_TAG_FLAGS           Tag = 7
	Tag_TAG_END             Tag = 255
)

// Enum value maps for Tag.
var (
	Tag_name = map[int32]string{
		0:   "TAG_SIGNATURE_TYPE",
		1:   "TAG_DOMAIN",
		2:   "TAG_PERSONALIZATION",
		3:   "TAG_EPOCH",
		4:   "TAG_EXPIRES_AT",
		5:   "TAG_COUNTER",
		6:   "TAG_CHALLENGE",
		7:   "TAG_FLAGS",
		255: "TAG_END",
	}
	Tag_value = map[string]int32{
		"TAG_SIGNATURE_TYPE":  0,
		"TAG_DOMAIN":          1,
		"TAG_PERSONALIZATION": 2,
		"TAG_EPOCH":           3,
		"TAG_EXPIRES_AT":      4,
		"TAG_COUNTER":         5,
		"TAG_CHALLENGE":       6,
		"TAG_FLAGS":           7,
		"TAG_END":             255,
	}
)

func (x Tag) Enum() *Tag {
	p := new(Tag)
	*p = x
	return p
}

func (x Tag) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Tag) Descriptor() protoreflect.EnumDescriptor {
	return file_signatures_proto_enumTypes[0].Descriptor()
}

func (Tag) Type() protoreflect.EnumType {
	return &file_signatures_proto_enumTypes[0]
}

func (x Tag) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Tag.Descriptor instead.
func (Tag) EnumDescriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{0}
}

type SignatureType int32

const (
	SignatureType_SIGNATURE_TYPE_AES_GCM              SignatureType = 0
	SignatureType_SIGNATURE_TYPE_AES_GCM_PERSONALIZED SignatureType = 5
	SignatureType_SIGNATURE_TYPE_HMAC                 SignatureType = 6
	SignatureType_SIGNATURE_TYPE_HMAC_PERSONALIZED    SignatureType = 8
)

// Enum value maps for SignatureType.
var (
	SignatureType_name = map[int32]string{
		0: "SIGNATURE_TYPE_AES_GCM",
		5: "SIGNATURE_TYPE_AES_GCM_PERSONALIZED",
		6: "SIGNATURE_TYPE_HMAC",
		8: "SIGNATURE_TYPE_HMAC_PERSONALIZED",
	}
	SignatureType_value = map[string]int32{
		"SIGNATURE_TYPE_AES_GCM":              0,
		"SIGNATURE_TYPE_AES_GCM_PERSONALIZED": 5,
		"SIGNATURE_TYPE_HMAC":                 6,
		"SIGNATURE_TYPE_HMAC_PERSONALIZED":    8,
	}
)

func (x SignatureType) Enum() *SignatureType {
	p := new(SignatureType)
	*p = x
	return p
}

func (x SignatureType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SignatureType) Descriptor() protoreflect.EnumDescriptor {
	return file_signatures_proto_enumTypes[1].Descriptor()
}

func (SignatureType) Type() protoreflect.EnumType {
	return &file_signatures_proto_enumTypes[1]
}

func (x SignatureType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SignatureType.Descriptor instead.
func (SignatureType) EnumDescriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{1}
}

type Session_Info_Status int32

const (
	Session_Info_Status_SESSION_INFO_STATUS_OK                   Session_Info_Status = 0
	Session_Info_Status_SESSION_INFO_STATUS_KEY_NOT_ON_WHITELIST Session_Info_Status = 1
)

// Enum value maps for Session_Info_Status.
var (
	Session_Info_Status_name = map[int32]string{
		0: "SESSION_INFO_STATUS_OK",
		1: "SESSION_INFO_STATUS_KEY_NOT_ON_WHITELIST",
	}
	Session_Info_Status_value = map[string]int32{
		"SESSION_INFO_STATUS_OK":                   0,
		"SESSION_INFO_STATUS_KEY_NOT_ON_WHITELIST": 1,
	}
)

func (x Session_Info_Status) Enum() *Session_Info_Status {
	p := new(Session_Info_Status)
	*p = x
	return p
}

func (x Session_Info_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Session_Info_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_signatures_proto_enumTypes[2].Descriptor()
}

func (Session_Info_Status) Type() protoreflect.EnumType {
	return &file_signatures_proto_enumTypes[2]
}

func (x Session_Info_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Session_Info_Status.Descriptor instead.
func (Session_Info_Status) EnumDescriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{2}
}

type KeyIdentity struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to IdentityType:
	//
	//	*KeyIdentity_PublicKey
	//	*KeyIdentity_Handle
	IdentityType isKeyIdentity_IdentityType `protobuf_oneof:"identity_type"`
}

func (x *KeyIdentity) Reset() {
	*x = KeyIdentity{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KeyIdentity) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KeyIdentity) ProtoMessage() {}

func (x *KeyIdentity) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KeyIdentity.ProtoReflect.Descriptor instead.
func (*KeyIdentity) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{0}
}

func (m *KeyIdentity) GetIdentityType() isKeyIdentity_IdentityType {
	if m != nil {
		return m.IdentityType
	}
	return nil
}

func (x *KeyIdentity) GetPublicKey() []byte {
	if x, ok := x.GetIdentityType().(*KeyIdentity_PublicKey); ok {
		return x.PublicKey
	}
	return nil
}

func (x *KeyIdentity) GetHandle() uint32 {
	if x, ok := x.GetIdentityType().(*KeyIdentity_Handle); ok {
		return x.Handle
	}
	return 0
}

type isKeyIdentity_IdentityType interface {
	isKeyIdentity_IdentityType()
}

type KeyIdentity_PublicKey struct {
	PublicKey []byte `protobuf:"bytes,1,opt,name=public_key,json=publicKey,proto3,oneof"`
}

type KeyIdentity_Handle struct {
	Handle uint32 `protobuf:"varint,3,opt,name=handle,proto3,oneof"`
}

func (*KeyIdentity_PublicKey) isKeyIdentity_IdentityType() {}

func (*KeyIdentity_Handle) isKeyIdentity_IdentityType() {}

type AES_GCM_Personalized_Signature_Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Epoch     []byte `protobuf:"bytes,1,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Nonce     []byte `protobuf:"bytes,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Counter   uint32 `protobuf:"varint,3,opt,name=counter,proto3" json:"counter,omitempty"`
	ExpiresAt uint32 `protobuf:"fixed32,4,opt,name=expires_at,json=expiresAt,proto3" json:"expires_at,omitempty"`
	Tag       []byte `protobuf:"bytes,5,opt,name=tag,proto3" json:"tag,omitempty"`
}

func (x *AES_GCM_Personalized_Signature_Data) Reset() {
	*x = AES_GCM_Personalized_Signature_Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AES_GCM_Personalized_Signature_Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AES_GCM_Personalized_Signature_Data) ProtoMessage() {}

func (x *AES_GCM_Personalized_Signature_Data) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AES_GCM_Personalized_Signature_Data.ProtoReflect.Descriptor instead.
func (*AES_GCM_Personalized_Signature_Data) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{1}
}

func (x *AES_GCM_Personalized_Signature_Data) GetEpoch() []byte {
	if x != nil {
		return x.Epoch
	}
	return nil
}

func (x *AES_GCM_Personalized_Signature_Data) GetNonce() []byte {
	if x != nil {
		return x.Nonce
	}
	return nil
}

func (x *AES_GCM_Personalized_Signature_Data) GetCounter() uint32 {
	if x != nil {
		return x.Counter
	}
	return 0
}

func (x *AES_GCM_Personalized_Signature_Data) GetExpiresAt() uint32 {
	if x != nil {
		return x.ExpiresAt
	}
	return 0
}

func (x *AES_GCM_Personalized_Signature_Data) GetTag() []byte {
	if x != nil {
		return x.Tag
	}
	return nil
}

type HMAC_Signature_Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tag []byte `protobuf:"bytes,1,opt,name=tag,proto3" json:"tag,omitempty"`
}

func (x *HMAC_Signature_Data) Reset() {
	*x = HMAC_Signature_Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HMAC_Signature_Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HMAC_Signature_Data) ProtoMessage() {}

func (x *HMAC_Signature_Data) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HMAC_Signature_Data.ProtoReflect.Descriptor instead.
func (*HMAC_Signature_Data) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{2}
}

func (x *HMAC_Signature_Data) GetTag() []byte {
	if x != nil {
		return x.Tag
	}
	return nil
}

type HMAC_Personalized_Signature_Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Epoch     []byte `protobuf:"bytes,1,opt,name=epoch,proto3" json:"epoch,omitempty"`
	Counter   uint32 `protobuf:"varint,2,opt,name=counter,proto3" json:"counter,omitempty"`
	ExpiresAt uint32 `protobuf:"fixed32,3,opt,name=expires_at,json=expiresAt,proto3" json:"expires_at,omitempty"`
	Tag       []byte `protobuf:"bytes,4,opt,name=tag,proto3" json:"tag,omitempty"`
}

func (x *HMAC_Personalized_Signature_Data) Reset() {
	*x = HMAC_Personalized_Signature_Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HMAC_Personalized_Signature_Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HMAC_Personalized_Signature_Data) ProtoMessage() {}

func (x *HMAC_Personalized_Signature_Data) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HMAC_Personalized_Signature_Data.ProtoReflect.Descriptor instead.
func (*HMAC_Personalized_Signature_Data) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{3}
}

func (x *HMAC_Personalized_Signature_Data) GetEpoch() []byte {
	if x != nil {
		return x.Epoch
	}
	return nil
}

func (x *HMAC_Personalized_Signature_Data) GetCounter() uint32 {
	if x != nil {
		return x.Counter
	}
	return 0
}

func (x *HMAC_Personalized_Signature_Data) GetExpiresAt() uint32 {
	if x != nil {
		return x.ExpiresAt
	}
	return 0
}

func (x *HMAC_Personalized_Signature_Data) GetTag() []byte {
	if x != nil {
		return x.Tag
	}
	return nil
}

type SignatureData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SignerIdentity *KeyIdentity `protobuf:"bytes,1,opt,name=signer_identity,json=signerIdentity,proto3" json:"signer_identity,omitempty"`
	// Types that are assignable to SigType:
	//
	//	*SignatureData_AES_GCM_PersonalizedData
	//	*SignatureData_SessionInfoTag
	//	*SignatureData_HMAC_PersonalizedData
	SigType isSignatureData_SigType `protobuf_oneof:"sig_type"`
}

func (x *SignatureData) Reset() {
	*x = SignatureData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignatureData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignatureData) ProtoMessage() {}

func (x *SignatureData) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignatureData.ProtoReflect.Descriptor instead.
func (*SignatureData) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{4}
}

func (x *SignatureData) GetSignerIdentity() *KeyIdentity {
	if x != nil {
		return x.SignerIdentity
	}
	return nil
}

func (m *SignatureData) GetSigType() isSignatureData_SigType {
	if m != nil {
		return m.SigType
	}
	return nil
}

func (x *SignatureData) GetAES_GCM_PersonalizedData() *AES_GCM_Personalized_Signature_Data {
	if x, ok := x.GetSigType().(*SignatureData_AES_GCM_PersonalizedData); ok {
		return x.AES_GCM_PersonalizedData
	}
	return nil
}

func (x *SignatureData) GetSessionInfoTag() *HMAC_Signature_Data {
	if x, ok := x.GetSigType().(*SignatureData_SessionInfoTag); ok {
		return x.SessionInfoTag
	}
	return nil
}

func (x *SignatureData) GetHMAC_PersonalizedData() *HMAC_Personalized_Signature_Data {
	if x, ok := x.GetSigType().(*SignatureData_HMAC_PersonalizedData); ok {
		return x.HMAC_PersonalizedData
	}
	return nil
}

type isSignatureData_SigType interface {
	isSignatureData_SigType()
}

type SignatureData_AES_GCM_PersonalizedData struct {
	AES_GCM_PersonalizedData *AES_GCM_Personalized_Signature_Data `protobuf:"bytes,5,opt,name=AES_GCM_Personalized_data,json=AESGCMPersonalizedData,proto3,oneof"`
}

type SignatureData_SessionInfoTag struct {
	SessionInfoTag *HMAC_Signature_Data `protobuf:"bytes,6,opt,name=session_info_tag,json=sessionInfoTag,proto3,oneof"`
}

type SignatureData_HMAC_PersonalizedData struct {
	HMAC_PersonalizedData *HMAC_Personalized_Signature_Data `protobuf:"bytes,8,opt,name=HMAC_Personalized_data,json=HMACPersonalizedData,proto3,oneof"`
}

func (*SignatureData_AES_GCM_PersonalizedData) isSignatureData_SigType() {}

func (*SignatureData_SessionInfoTag) isSignatureData_SigType() {}

func (*SignatureData_HMAC_PersonalizedData) isSignatureData_SigType() {}

type GetSessionInfoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KeyIdentity *KeyIdentity `protobuf:"bytes,1,opt,name=key_identity,json=keyIdentity,proto3" json:"key_identity,omitempty"`
}

func (x *GetSessionInfoRequest) Reset() {
	*x = GetSessionInfoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSessionInfoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSessionInfoRequest) ProtoMessage() {}

func (x *GetSessionInfoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSessionInfoRequest.ProtoReflect.Descriptor instead.
func (*GetSessionInfoRequest) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{5}
}

func (x *GetSessionInfoRequest) GetKeyIdentity() *KeyIdentity {
	if x != nil {
		return x.KeyIdentity
	}
	return nil
}

type SessionInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Counter   uint32              `protobuf:"varint,1,opt,name=counter,proto3" json:"counter,omitempty"`
	PublicKey []byte              `protobuf:"bytes,2,opt,name=publicKey,proto3" json:"publicKey,omitempty"`
	Epoch     []byte              `protobuf:"bytes,3,opt,name=epoch,proto3" json:"epoch,omitempty"`
	ClockTime uint32              `protobuf:"fixed32,4,opt,name=clock_time,json=clockTime,proto3" json:"clock_time,omitempty"`
	Status    Session_Info_Status `protobuf:"varint,5,opt,name=status,proto3,enum=Signatures.Session_Info_Status" json:"status,omitempty"`
	Handle    uint32              `protobuf:"varint,6,opt,name=handle,proto3" json:"handle,omitempty"`
}

func (x *SessionInfo) Reset() {
	*x = SessionInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_signatures_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SessionInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SessionInfo) ProtoMessage() {}

func (x *SessionInfo) ProtoReflect() protoreflect.Message {
	mi := &file_signatures_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SessionInfo.ProtoReflect.Descriptor instead.
func (*SessionInfo) Descriptor() ([]byte, []int) {
	return file_signatures_proto_rawDescGZIP(), []int{6}
}

func (x *SessionInfo) GetCounter() uint32 {
	if x != nil {
		return x.Counter
	}
	return 0
}

func (x *SessionInfo) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *SessionInfo) GetEpoch() []byte {
	if x != nil {
		return x.Epoch
	}
	return nil
}

func (x *SessionInfo) GetClockTime() uint32 {
	if x != nil {
		return x.ClockTime
	}
	return 0
}

func (x *SessionInfo) GetStatus() Session_Info_Status {
	if x != nil {
		return x.Status
	}
	return Session_Info_Status_SESSION_INFO_STATUS_OK
}

func (x *SessionInfo) GetHandle() uint32 {
	if x != nil {
		return x.Handle
	}
	return 0
}

var File_signatures_proto protoreflect.FileDescriptor

var file_signatures_proto_rawDesc = []byte{
	0x0a, 0x10, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x0a, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x22, 0x5f,
	0x0a, 0x0b, 0x4b, 0x65, 0x79, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x12, 0x1f, 0x0a,
	0x0a, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0c, 0x48, 0x00, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x12, 0x18,
	0x0a, 0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x48, 0x00,
	0x52, 0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x42, 0x0f, 0x0a, 0x0d, 0x69, 0x64, 0x65, 0x6e,
	0x74, 0x69, 0x74, 0x79, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x4a, 0x04, 0x08, 0x02, 0x10, 0x03, 0x22,
	0x9c, 0x01, 0x0a, 0x23, 0x41, 0x45, 0x53, 0x5f, 0x47, 0x43, 0x4d, 0x5f, 0x50, 0x65, 0x72, 0x73,
	0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x5f, 0x44, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x12, 0x14, 0x0a,
	0x05, 0x6e, 0x6f, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x6e, 0x6f,
	0x6e, 0x63, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x1d, 0x0a,
	0x0a, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x5f, 0x61, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x07, 0x52, 0x09, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x41, 0x74, 0x12, 0x10, 0x0a, 0x03,
	0x74, 0x61, 0x67, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x74, 0x61, 0x67, 0x22, 0x27,
	0x0a, 0x13, 0x48, 0x4d, 0x41, 0x43, 0x5f, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x5f, 0x44, 0x61, 0x74, 0x61, 0x12, 0x10, 0x0a, 0x03, 0x74, 0x61, 0x67, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x03, 0x74, 0x61, 0x67, 0x22, 0x83, 0x01, 0x0a, 0x20, 0x48, 0x4d, 0x41, 0x43,
	0x5f, 0x50, 0x65, 0x72, 0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x53, 0x69,
	0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x44, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05,
	0x65, 0x70, 0x6f, 0x63, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x65, 0x70, 0x6f,
	0x63, 0x68, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0d, 0x52, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x1d, 0x0a, 0x0a,
	0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x07,
	0x52, 0x09, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x41, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x74,
	0x61, 0x67, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x74, 0x61, 0x67, 0x22, 0x84, 0x03,
	0x0a, 0x0d, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x44, 0x61, 0x74, 0x61, 0x12,
	0x40, 0x0a, 0x0f, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x74, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x4b, 0x65, 0x79, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74,
	0x79, 0x52, 0x0e, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74,
	0x79, 0x12, 0x6c, 0x0a, 0x19, 0x41, 0x45, 0x53, 0x5f, 0x47, 0x43, 0x4d, 0x5f, 0x50, 0x65, 0x72,
	0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x73, 0x2e, 0x41, 0x45, 0x53, 0x5f, 0x47, 0x43, 0x4d, 0x5f, 0x50, 0x65, 0x72, 0x73, 0x6f, 0x6e,
	0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x5f, 0x44, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x16, 0x41, 0x45, 0x53, 0x47, 0x43, 0x4d, 0x50,
	0x65, 0x72, 0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x44, 0x61, 0x74, 0x61, 0x12,
	0x4b, 0x0a, 0x10, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x6e, 0x66, 0x6f, 0x5f,
	0x74, 0x61, 0x67, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x53, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x48, 0x4d, 0x41, 0x43, 0x5f, 0x53, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x44, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x0e, 0x73, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x54, 0x61, 0x67, 0x12, 0x64, 0x0a, 0x16,
	0x48, 0x4d, 0x41, 0x43, 0x5f, 0x50, 0x65, 0x72, 0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65,
	0x64, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x53,
	0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x48, 0x4d, 0x41, 0x43, 0x5f, 0x50,
	0x65, 0x72, 0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x53, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x44, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x14, 0x48, 0x4d,
	0x41, 0x43, 0x50, 0x65, 0x72, 0x73, 0x6f, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x44, 0x61,
	0x74, 0x61, 0x42, 0x0a, 0x0a, 0x08, 0x73, 0x69, 0x67, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x4a, 0x04,
	0x08, 0x07, 0x10, 0x08, 0x22, 0x53, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x53, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3a, 0x0a,
	0x0c, 0x6b, 0x65, 0x79, 0x5f, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73,
	0x2e, 0x4b, 0x65, 0x79, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x52, 0x0b, 0x6b, 0x65,
	0x79, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x22, 0xcb, 0x01, 0x0a, 0x0b, 0x53, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x75,
	0x6e, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x65, 0x72, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6c, 0x6f, 0x63, 0x6b,
	0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x07, 0x52, 0x09, 0x63, 0x6c, 0x6f,
	0x63, 0x6b, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x37, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x1f, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x73, 0x2e, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x49, 0x6e, 0x66, 0x6f,
	0x5f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12,
	0x16, 0x0a, 0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x06, 0x68, 0x61, 0x6e, 0x64, 0x6c, 0x65, 0x2a, 0xaa, 0x01, 0x0a, 0x03, 0x54, 0x61, 0x67, 0x12,
	0x16, 0x0a, 0x12, 0x54, 0x41, 0x47, 0x5f, 0x53, 0x49, 0x47, 0x4e, 0x41, 0x54, 0x55, 0x52, 0x45,
	0x5f, 0x54, 0x59, 0x50, 0x45, 0x10, 0x00, 0x12, 0x0e, 0x0a, 0x0a, 0x54, 0x41, 0x47, 0x5f, 0x44,
	0x4f, 0x4d, 0x41, 0x49, 0x4e, 0x10, 0x01, 0x12, 0x17, 0x0a, 0x13, 0x54, 0x41, 0x47, 0x5f, 0x50,
	0x45, 0x52, 0x53, 0x4f, 0x4e, 0x41, 0x4c, 0x49, 0x5a, 0x41, 0x54, 0x49, 0x4f, 0x4e, 0x10, 0x02,
	0x12, 0x0d, 0x0a, 0x09, 0x54, 0x41, 0x47, 0x5f, 0x45, 0x50, 0x4f, 0x43, 0x48, 0x10, 0x03, 0x12,
	0x12, 0x0a, 0x0e, 0x54, 0x41, 0x47, 0x5f, 0x45, 0x58, 0x50, 0x49, 0x52, 0x45, 0x53, 0x5f, 0x41,
	0x54, 0x10, 0x04, 0x12, 0x0f, 0x0a, 0x0b, 0x54, 0x41, 0x47, 0x5f, 0x43, 0x4f, 0x55, 0x4e, 0x54,
	0x45, 0x52, 0x10, 0x05, 0x12, 0x11, 0x0a, 0x0d, 0x54, 0x41, 0x47, 0x5f, 0x43, 0x48, 0x41, 0x4c,
	0x4c, 0x45, 0x4e, 0x47, 0x45, 0x10, 0x06, 0x12, 0x0d, 0x0a, 0x09, 0x54, 0x41, 0x47, 0x5f, 0x46,
	0x4c, 0x41, 0x47, 0x53, 0x10, 0x07, 0x12, 0x0c, 0x0a, 0x07, 0x54, 0x41, 0x47, 0x5f, 0x45, 0x4e,
	0x44, 0x10, 0xff, 0x01, 0x2a, 0x99, 0x01, 0x0a, 0x0d, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x1a, 0x0a, 0x16, 0x53, 0x49, 0x47, 0x4e, 0x41, 0x54,
	0x55, 0x52, 0x45, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x41, 0x45, 0x53, 0x5f, 0x47, 0x43, 0x4d,
	0x10, 0x00, 0x12, 0x27, 0x0a, 0x23, 0x53, 0x49, 0x47, 0x4e, 0x41, 0x54, 0x55, 0x52, 0x45, 0x5f,
	0x54, 0x59, 0x50, 0x45, 0x5f, 0x41, 0x45, 0x53, 0x5f, 0x47, 0x43, 0x4d, 0x5f, 0x50, 0x45, 0x52,
	0x53, 0x4f, 0x4e, 0x41, 0x4c, 0x49, 0x5a, 0x45, 0x44, 0x10, 0x05, 0x12, 0x17, 0x0a, 0x13, 0x53,
	0x49, 0x47, 0x4e, 0x41, 0x54, 0x55, 0x52, 0x45, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x48, 0x4d,
	0x41, 0x43, 0x10, 0x06, 0x12, 0x24, 0x0a, 0x20, 0x53, 0x49, 0x47, 0x4e, 0x41, 0x54, 0x55, 0x52,
	0x45, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x48, 0x4d, 0x41, 0x43, 0x5f, 0x50, 0x45, 0x52, 0x53,
	0x4f, 0x4e, 0x41, 0x4c, 0x49, 0x5a, 0x45, 0x44, 0x10, 0x08, 0x22, 0x04, 0x08, 0x07, 0x10, 0x07,
	0x2a, 0x5f, 0x0a, 0x13, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x49, 0x6e, 0x66, 0x6f,
	0x5f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x1a, 0x0a, 0x16, 0x53, 0x45, 0x53, 0x53, 0x49,
	0x4f, 0x4e, 0x5f, 0x49, 0x4e, 0x46, 0x4f, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x4f,
	0x4b, 0x10, 0x00, 0x12, 0x2c, 0x0a, 0x28, 0x53, 0x45, 0x53, 0x53, 0x49, 0x4f, 0x4e, 0x5f, 0x49,
	0x4e, 0x46, 0x4f, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x4b, 0x45, 0x59, 0x5f, 0x4e,
	0x4f, 0x54, 0x5f, 0x4f, 0x4e, 0x5f, 0x57, 0x48, 0x49, 0x54, 0x45, 0x4c, 0x49, 0x53, 0x54, 0x10,
	0x01, 0x42, 0x69, 0x0a, 0x1e, 0x63, 0x6f, 0x6d, 0x2e, 0x74, 0x65, 0x73, 0x6c, 0x61, 0x2e, 0x67,
	0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x73, 0x5a, 0x47, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x74, 0x65, 0x73, 0x6c, 0x61, 0x6d, 0x6f, 0x74, 0x6f, 0x72, 0x73, 0x2f, 0x76, 0x65, 0x68, 0x69,
	0x63, 0x6c, 0x65, 0x2d, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_signatures_proto_rawDescOnce sync.Once
	file_signatures_proto_rawDescData = file_signatures_proto_rawDesc
)

func file_signatures_proto_rawDescGZIP() []byte {
	file_signatures_proto_rawDescOnce.Do(func() {
		file_signatures_proto_rawDescData = protoimpl.X.CompressGZIP(file_signatures_proto_rawDescData)
	})
	return file_signatures_proto_rawDescData
}

var file_signatures_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_signatures_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_signatures_proto_goTypes = []interface{}{
	(Tag)(0),                 // 0: Signatures.Tag
	(SignatureType)(0),       // 1: Signatures.SignatureType
	(Session_Info_Status)(0), // 2: Signatures.Session_Info_Status
	(*KeyIdentity)(nil),      // 3: Signatures.KeyIdentity
	(*AES_GCM_Personalized_Signature_Data)(nil), // 4: Signatures.AES_GCM_Personalized_Signature_Data
	(*HMAC_Signature_Data)(nil),                 // 5: Signatures.HMAC_Signature_Data
	(*HMAC_Personalized_Signature_Data)(nil),    // 6: Signatures.HMAC_Personalized_Signature_Data
	(*SignatureData)(nil),                       // 7: Signatures.SignatureData
	(*GetSessionInfoRequest)(nil),               // 8: Signatures.GetSessionInfoRequest
	(*SessionInfo)(nil),                         // 9: Signatures.SessionInfo
}
var file_signatures_proto_depIdxs = []int32{
	3, // 0: Signatures.SignatureData.signer_identity:type_name -> Signatures.KeyIdentity
	4, // 1: Signatures.SignatureData.AES_GCM_Personalized_data:type_name -> Signatures.AES_GCM_Personalized_Signature_Data
	5, // 2: Signatures.SignatureData.session_info_tag:type_name -> Signatures.HMAC_Signature_Data
	6, // 3: Signatures.SignatureData.HMAC_Personalized_data:type_name -> Signatures.HMAC_Personalized_Signature_Data
	3, // 4: Signatures.GetSessionInfoRequest.key_identity:type_name -> Signatures.KeyIdentity
	2, // 5: Signatures.SessionInfo.status:type_name -> Signatures.Session_Info_Status
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_signatures_proto_init() }
func file_signatures_proto_init() {
	if File_signatures_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_signatures_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*KeyIdentity); i {
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
		file_signatures_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AES_GCM_Personalized_Signature_Data); i {
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
		file_signatures_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HMAC_Signature_Data); i {
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
		file_signatures_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HMAC_Personalized_Signature_Data); i {
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
		file_signatures_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignatureData); i {
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
		file_signatures_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSessionInfoRequest); i {
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
		file_signatures_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SessionInfo); i {
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
	file_signatures_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*KeyIdentity_PublicKey)(nil),
		(*KeyIdentity_Handle)(nil),
	}
	file_signatures_proto_msgTypes[4].OneofWrappers = []interface{}{
		(*SignatureData_AES_GCM_PersonalizedData)(nil),
		(*SignatureData_SessionInfoTag)(nil),
		(*SignatureData_HMAC_PersonalizedData)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_signatures_proto_rawDesc,
			NumEnums:      3,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_signatures_proto_goTypes,
		DependencyIndexes: file_signatures_proto_depIdxs,
		EnumInfos:         file_signatures_proto_enumTypes,
		MessageInfos:      file_signatures_proto_msgTypes,
	}.Build()
	File_signatures_proto = out.File
	file_signatures_proto_rawDesc = nil
	file_signatures_proto_goTypes = nil
	file_signatures_proto_depIdxs = nil
}
