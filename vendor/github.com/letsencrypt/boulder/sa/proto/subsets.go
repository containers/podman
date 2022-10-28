// Copied from the auto-generated sa_grpc.pb.go

package proto

import (
	context "context"

	proto "github.com/letsencrypt/boulder/core/proto"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// StorageAuthorityGetterClient is a read-only subset of the sapb.StorageAuthorityClient interface
type StorageAuthorityGetterClient interface {
	GetRegistration(ctx context.Context, in *RegistrationID, opts ...grpc.CallOption) (*proto.Registration, error)
	GetRegistrationByKey(ctx context.Context, in *JSONWebKey, opts ...grpc.CallOption) (*proto.Registration, error)
	GetCertificate(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*proto.Certificate, error)
	GetPrecertificate(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*proto.Certificate, error)
	GetCertificateStatus(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*proto.CertificateStatus, error)
	CountCertificatesByNames(ctx context.Context, in *CountCertificatesByNamesRequest, opts ...grpc.CallOption) (*CountByNames, error)
	CountRegistrationsByIP(ctx context.Context, in *CountRegistrationsByIPRequest, opts ...grpc.CallOption) (*Count, error)
	CountRegistrationsByIPRange(ctx context.Context, in *CountRegistrationsByIPRequest, opts ...grpc.CallOption) (*Count, error)
	CountOrders(ctx context.Context, in *CountOrdersRequest, opts ...grpc.CallOption) (*Count, error)
	CountFQDNSets(ctx context.Context, in *CountFQDNSetsRequest, opts ...grpc.CallOption) (*Count, error)
	FQDNSetExists(ctx context.Context, in *FQDNSetExistsRequest, opts ...grpc.CallOption) (*Exists, error)
	PreviousCertificateExists(ctx context.Context, in *PreviousCertificateExistsRequest, opts ...grpc.CallOption) (*Exists, error)
	GetAuthorization2(ctx context.Context, in *AuthorizationID2, opts ...grpc.CallOption) (*proto.Authorization, error)
	GetAuthorizations2(ctx context.Context, in *GetAuthorizationsRequest, opts ...grpc.CallOption) (*Authorizations, error)
	GetPendingAuthorization2(ctx context.Context, in *GetPendingAuthorizationRequest, opts ...grpc.CallOption) (*proto.Authorization, error)
	CountPendingAuthorizations2(ctx context.Context, in *RegistrationID, opts ...grpc.CallOption) (*Count, error)
	GetValidOrderAuthorizations2(ctx context.Context, in *GetValidOrderAuthorizationsRequest, opts ...grpc.CallOption) (*Authorizations, error)
	CountInvalidAuthorizations2(ctx context.Context, in *CountInvalidAuthorizationsRequest, opts ...grpc.CallOption) (*Count, error)
	GetValidAuthorizations2(ctx context.Context, in *GetValidAuthorizationsRequest, opts ...grpc.CallOption) (*Authorizations, error)
	KeyBlocked(ctx context.Context, in *KeyBlockedRequest, opts ...grpc.CallOption) (*Exists, error)
	GetOrder(ctx context.Context, in *OrderRequest, opts ...grpc.CallOption) (*proto.Order, error)
	GetOrderForNames(ctx context.Context, in *GetOrderForNamesRequest, opts ...grpc.CallOption) (*proto.Order, error)
	IncidentsForSerial(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*Incidents, error)
}

// StorageAuthorityCertificateClient is a subset of the sapb.StorageAuthorityClient interface that only reads and writes certificates
type StorageAuthorityCertificateClient interface {
	AddSerial(ctx context.Context, in *AddSerialRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	AddPrecertificate(ctx context.Context, in *AddCertificateRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetPrecertificate(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*proto.Certificate, error)
	AddCertificate(ctx context.Context, in *AddCertificateRequest, opts ...grpc.CallOption) (*AddCertificateResponse, error)
	GetCertificate(ctx context.Context, in *Serial, opts ...grpc.CallOption) (*proto.Certificate, error)
}
