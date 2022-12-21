/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package universalapi

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contribCrypto "github.com/dapr/components-contrib/crypto"
	diag "github.com/dapr/dapr/pkg/diagnostics"
	"github.com/dapr/dapr/pkg/messages"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/pkg/resiliency"
)

// SubtleGetKeyAlpha1 returns the public part of an asymmetric key stored in the vault.
func (a *UniversalAPI) SubtleGetKeyAlpha1(ctx context.Context, in *runtimev1pb.SubtleGetKeyAlpha1Request) (*runtimev1pb.SubtleGetKeyAlpha1Response, error) {
	component, err := a.cryptoValidateRequest(in.ComponentName)
	if err != nil {
		return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
	}

	// Get the key
	policyRunner := resiliency.NewRunner[jwk.Key](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.ComponentName, resiliency.Crypto),
	)
	start := time.Now()
	res, err := policyRunner(func(ctx context.Context) (jwk.Key, error) {
		return component.GetKey(ctx, in.Name)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.CryptoInvoked(ctx, in.ComponentName, diag.Get, err == nil, elapsed)

	if err != nil {
		err = status.Errorf(codes.Internal, messages.ErrCryptoGetKey, in.Name, err.Error())
		a.Logger.Debug(err)
		return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
	}

	// Get the key ID if present
	kid := in.Name
	if dk, ok := res.(*contribCrypto.Key); ok {
		kid = dk.KeyID()
	}

	// Format the response
	var pk []byte
	switch in.Format {
	case runtimev1pb.SubtleGetKeyAlpha1Request_PEM: //nolint:nosnakecase
		var (
			v   crypto.PublicKey
			der []byte
		)
		err = res.Raw(&v)
		if err != nil {
			err = status.Errorf(codes.Internal, "failed to marshal public key %s as PKIX: %s", in.Name, err.Error())
			a.Logger.Debug(err)
			return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
		}
		der, err = x509.MarshalPKIXPublicKey(v)
		if err != nil {
			err = status.Errorf(codes.Internal, "failed to marshal public key %s as PKIX: %s", in.Name, err.Error())
			a.Logger.Debug(err)
			return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
		}
		pk = pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})

	case runtimev1pb.SubtleGetKeyAlpha1Request_JSON: //nolint:nosnakecase
		pk, err = json.Marshal(res)
		if err != nil {
			err = status.Errorf(codes.Internal, "failed to marshal public key %s as JSON: %s", in.Name, err.Error())
			a.Logger.Debug(err)
			return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
		}

	default:
		err = status.Errorf(codes.InvalidArgument, "invalid key format")
		a.Logger.Debug(err)
		return &runtimev1pb.SubtleGetKeyAlpha1Response{}, err
	}

	return &runtimev1pb.SubtleGetKeyAlpha1Response{
		Name:      kid,
		PublicKey: string(pk),
	}, nil
}

type subtleEncryptRes struct {
	ciphertext []byte
	tag        []byte
}

// SubtleEncryptAlpha1 encrypts a small message using a key stored in the vault.
func (a *UniversalAPI) SubtleEncryptAlpha1(ctx context.Context, in *runtimev1pb.SubtleEncryptAlpha1Request) (*runtimev1pb.SubtleEncryptAlpha1Response, error) {
	component, err := a.cryptoValidateRequest(in.ComponentName)
	if err != nil {
		return &runtimev1pb.SubtleEncryptAlpha1Response{}, err
	}

	policyRunner := resiliency.NewRunner[*subtleEncryptRes](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.ComponentName, resiliency.Crypto),
	)
	start := time.Now()
	ser, err := policyRunner(func(ctx context.Context) (*subtleEncryptRes, error) {
		ciphertext, tag, rErr := component.Encrypt(ctx, in.Plaintext, in.Algorithm, in.Key, in.Nonce, in.AssociatedData)
		if rErr != nil {
			return nil, rErr
		}
		return &subtleEncryptRes{
			ciphertext: ciphertext,
			tag:        tag,
		}, nil
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.CryptoInvoked(ctx, in.ComponentName, diag.CryptoOp, err == nil, elapsed)

	if err != nil {
		err = status.Errorf(codes.Internal, messages.ErrCryptoOperation, err.Error())
		a.Logger.Debug(err)
		return &runtimev1pb.SubtleEncryptAlpha1Response{}, err
	}

	if ser == nil {
		ser = &subtleEncryptRes{}
	}
	return &runtimev1pb.SubtleEncryptAlpha1Response{
		Ciphertext: ser.ciphertext,
		Tag:        ser.tag,
	}, nil
}

// SubtleDecryptAlpha1 decrypts a small message using a key stored in the vault.
func (a *UniversalAPI) SubtleDecryptAlpha1(ctx context.Context, in *runtimev1pb.SubtleDecryptAlpha1Request) (*runtimev1pb.SubtleDecryptAlpha1Response, error) {
	component, err := a.cryptoValidateRequest(in.ComponentName)
	if err != nil {
		return &runtimev1pb.SubtleDecryptAlpha1Response{}, err
	}

	policyRunner := resiliency.NewRunner[[]byte](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.ComponentName, resiliency.Crypto),
	)
	start := time.Now()
	plaintext, err := policyRunner(func(ctx context.Context) ([]byte, error) {
		return component.Decrypt(ctx, in.Ciphertext, in.Algorithm, in.Key, in.Nonce, in.Tag, in.AssociatedData)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.CryptoInvoked(ctx, in.ComponentName, diag.CryptoOp, err == nil, elapsed)

	if err != nil {
		err = status.Errorf(codes.Internal, messages.ErrCryptoOperation, err.Error())
		a.Logger.Debug(err)
		return &runtimev1pb.SubtleDecryptAlpha1Response{}, err
	}

	return &runtimev1pb.SubtleDecryptAlpha1Response{
		Plaintext: plaintext,
	}, nil
}

// Internal method that checks if the request is for a valid crypto component.
func (a *UniversalAPI) cryptoValidateRequest(componentName string) (contribCrypto.SubtleCrypto, error) {
	if a.CryptoProviders == nil || len(a.CryptoProviders) == 0 {
		err := status.Error(codes.FailedPrecondition, messages.ErrCryptoProvidersNotConfigured)
		a.Logger.Debug(err)
		return nil, err
	}

	component := a.CryptoProviders[componentName]
	if component == nil {
		err := status.Errorf(codes.InvalidArgument, messages.ErrCryptoProviderNotFound, componentName)
		a.Logger.Debug(err)
		return nil, err
	}

	return component, nil
}
